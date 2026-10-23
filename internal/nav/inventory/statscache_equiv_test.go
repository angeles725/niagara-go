package inventory

import (
	"bytes"
	"encoding/gob"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/angeles725/niagara-go/internal/atomicfile"
	"github.com/angeles725/niagara-go/internal/nav/core"
	"github.com/angeles725/niagara-go/internal/statcache"
)

// Equivalence with the code before internal/statcache: the legacy* functions
// are the hand-rolled read, stat-again check and write of the stats cache,
// verbatim, and every case must decide (and write) as they did.

func legacyOpenStatsCache(indexDir string) *statsCache {
	c := &statsCache{path: filepath.Join(indexDir, ".mnav-cache", statsCacheName), entries: map[string]statsEntry{}}
	data, err := os.ReadFile(c.path)
	if err != nil {
		return c
	}
	var f statsCacheFile
	if gob.NewDecoder(bytes.NewReader(data)).Decode(&f) == nil && f.Format == statsCacheFormat && f.Entries != nil {
		c.entries = f.Entries
	}
	return c
}

func (c *statsCache) legacySummary(path string, full bool) summary {
	st, err := os.Stat(path)
	if err != nil || !st.Mode().IsRegular() {
		return summary{}
	}
	key := core.ResolvedPath(path)
	cacheable := st.Size() >= statsCacheMinBytes
	if e, ok := c.entries[key]; ok && cacheable && e.Stamp == statcache.Of(st) && // The key is the one change since: the whole statcache.Stamp, ctime and inode too.

		(e.Summary.Full || !full) {
		return e.Summary
	}
	s := summarize(path, full)
	if cacheable {
		if st2, err := os.Stat(path); err == nil && st2.Size() == st.Size() && st2.ModTime().Equal(st.ModTime()) {
			c.entries[key] = statsEntry{statcache.Of(st), s}
			c.dirty = true
		}
	}
	return s
}

func (c *statsCache) legacySave() {
	if !c.dirty {
		return
	}
	for k := range c.entries {
		if _, err := os.Stat(k); err != nil {
			delete(c.entries, k)
		}
	}
	var buf bytes.Buffer
	if gob.NewEncoder(&buf).Encode(statsCacheFile{statsCacheFormat, c.entries}) != nil ||
		os.MkdirAll(filepath.Dir(c.path), 0o700) != nil {
		return
	}
	_ = atomicfile.WriteStream(c.path, 0o600, func(w io.Writer) error {
		_, err := w.Write(buf.Bytes())
		return err
	})
	c.dirty = false
}

func writeStatsCache(t *testing.T, dir string, v any) {
	t.Helper()
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(v); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(dir, ".mnav-cache", statsCacheName)
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil || os.WriteFile(p, buf.Bytes(), 0o600) != nil {
		t.Fatal(err)
	}
}

func TestOpenStatsCacheMatchesLegacy(t *testing.T) {
	entries := map[string]statsEntry{"/i/x.json": {statcache.Stamp{Size: 5, MtimeNs: 6}, summary{Loaded: true, Meta: []byte("{}")}}}
	cases := map[string]any{
		"valid":         statsCacheFile{statsCacheFormat, entries},
		"other format":  statsCacheFile{statsCacheFormat + 1, entries},
		"nil entries":   statsCacheFile{statsCacheFormat, nil},
		"another type":  []string{"x"},
		"garbage":       []byte("garbage"),
		"no cache file": nil,
	}
	for name, v := range cases {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			switch v := v.(type) {
			case nil:
			case []byte:
				p := filepath.Join(dir, ".mnav-cache", statsCacheName)
				if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil || os.WriteFile(p, v, 0o600) != nil {
					t.Fatal(err)
				}
			default:
				writeStatsCache(t, dir, v)
			}
			got, want := openStatsCache(dir), legacyOpenStatsCache(dir)
			if !reflect.DeepEqual(got, want) || (name == "valid") != (len(got.entries) == 1) {
				t.Fatalf("%+v, legacy %+v", got, want)
			}
		})
	}
}

// TestSummaryMatchesLegacy runs the same sequence of requests and file
// changes through the cache and through the legacy code: every answer, every
// entry and the dirty flag must agree at each step.
func TestSummaryMatchesLegacy(t *testing.T) {
	defer func(n int64) { statsCacheMinBytes = n }(statsCacheMinBytes)
	statsCacheMinBytes = 12
	dir := t.TempDir()
	big, small := filepath.Join(dir, "big.json"), filepath.Join(dir, "small.json")
	stamp := time.Date(2026, 1, 2, 3, 4, 5, 6, time.UTC)
	set := func(p, text string, mt time.Time) {
		write(t, p, text)
		if err := os.Chtimes(p, mt, mt); err != nil {
			t.Fatal(err)
		}
	}
	set(big, `{"_meta": {"a": 1}}`, stamp)
	set(small, `{"b": 2}`, stamp)
	steps := []struct {
		name   string
		change func()
		path   string
		full   bool
	}{
		{"cold prefix", func() {}, big, false},
		{"warm prefix", func() {}, big, false},
		{"full over a prefix entry", func() {}, big, true},
		{"prefix over a full entry", func() {}, big, false},
		{"same size and mtime", func() { set(big, `{"_meta": {"a": 9}}`, stamp) }, big, false},
		{"mtime +1s", func() { set(big, `{"_meta": {"a": 9}}`, stamp.Add(time.Second)) }, big, false},
		{"resized", func() { set(big, `{"_meta": {"a": 10}}`, stamp.Add(time.Second)) }, big, true},
		{"under the floor", func() {}, small, true},
		{"directory", func() {}, dir, false},
		{"missing", func() {}, filepath.Join(dir, "gone.json"), false},
	}
	c, l := openStatsCache(dir), legacyOpenStatsCache(dir)
	for _, s := range steps {
		s.change()
		got, want := c.summary(s.path, s.full), l.legacySummary(s.path, s.full)
		if !reflect.DeepEqual(got, want) || !reflect.DeepEqual(c.entries, l.entries) || c.dirty != l.dirty {
			t.Fatalf("%s: %+v (%d entries, dirty %v), legacy %+v (%d entries, dirty %v)",
				s.name, got, len(c.entries), c.dirty, want, len(l.entries), l.dirty)
		}
	}
	if _, ok := c.entries[core.ResolvedPath(small)]; ok {
		t.Error("a file under statsCacheMinBytes was cached")
	}
}

// TestSaveMatchesLegacy: save drops the entries of removed files as before
// and writes the bytes the legacy writer wrote (one entry: gob writes a map
// in iteration order), mode 0600.
func TestSaveMatchesLegacy(t *testing.T) {
	defer func(n int64) { statsCacheMinBytes = n }(statsCacheMinBytes)
	statsCacheMinBytes = 0
	newDir, oldDir := t.TempDir(), t.TempDir()
	// Both trees get the same fixed mtime: the two writes land microseconds apart, so the
	// entries would otherwise differ in MtimeNs on a fast clock (seen in CI).
	fixed := time.Unix(1700000000, 123456789)
	for _, d := range []string{newDir, oldDir} {
		for name, body := range map[string]string{"x.json": `{"_meta": {"a": 1}}`, "y.json": `{"_meta": {"b": 2}}`} {
			write(t, filepath.Join(d, name), body)
			if err := os.Chtimes(filepath.Join(d, name), fixed, fixed); err != nil {
				t.Fatal(err)
			}
		}
	}
	c, l := openStatsCache(newDir), legacyOpenStatsCache(oldDir)
	for _, name := range []string{"x.json", "y.json"} {
		c.summary(filepath.Join(newDir, name), true)
		l.legacySummary(filepath.Join(oldDir, name), true)
	}
	for _, d := range []string{newDir, oldDir} {
		if err := os.Remove(filepath.Join(d, "y.json")); err != nil {
			t.Fatal(err)
		}
	}
	c.save()
	l.legacySave()
	if len(c.entries) != 1 || len(l.entries) != 1 || c.dirty || l.dirty {
		t.Fatalf("after save: %d entries dirty %v, legacy %d entries dirty %v", len(c.entries), c.dirty, len(l.entries), l.dirty)
	}
	got := readFile(t, c.path)
	// The legacy file keys on oldDir's path: rewrite it under newDir's key to compare bytes.
	var f statsCacheFile
	if err := gob.NewDecoder(bytes.NewReader(readFile(t, l.path))).Decode(&f); err != nil {
		t.Fatal(err)
	}
	moved := map[string]statsEntry{}
	for k, e := range f.Entries {
		rel, _ := filepath.Rel(core.ResolvedPath(oldDir), k)
		k := filepath.Join(core.ResolvedPath(newDir), rel)
		// Another file of the same size and mtime: its own inode and ctime.
		e.CtimeNs, e.Ino = c.entries[k].CtimeNs, c.entries[k].Ino
		moved[k] = e
	}
	l.entries, l.dirty, l.path = moved, true, filepath.Join(t.TempDir(), statsCacheName)
	if want := c.entries; !reflect.DeepEqual(moved, want) {
		t.Fatalf("entries %+v, legacy %+v", want, moved)
	}
	l.legacySave()
	if !bytes.Equal(got, readFile(t, l.path)) {
		t.Fatal("cache bytes differ from the legacy writer")
	}
	if st, err := os.Stat(c.path); err != nil || st.Mode().Perm() != 0o600 {
		t.Fatalf("cache mode %v, %v", st.Mode(), err)
	}
	for _, name := range []string{"statsCacheFile", "statsEntry", "summary", "MtimeNs"} {
		if !bytes.Contains(got, []byte(name)) {
			t.Errorf("cache bytes do not name %s", name)
		}
	}
}

func readFile(t *testing.T, p string) []byte {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
