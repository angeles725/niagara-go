package callgraph

import (
	"bytes"
	"encoding/gob"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/angeles725/niagara-go/internal/atomicfile"
	"github.com/angeles725/niagara-go/internal/nav/core"
	"github.com/angeles725/niagara-go/internal/statcache"
)

// Equivalence with the code before internal/statcache (PR #179): the legacy*
// functions are that code verbatim, and every case must decide as it did.

func legacyNewCache(indexPath, cachePath string) *Cache {
	st, err := os.Stat(indexPath)
	if err != nil || !st.Mode().IsRegular() || st.Size() < cacheMinBytes {
		return &Cache{}
	}
	// The key is the one change since: the whole statcache.Stamp, ctime and inode too.
	return &Cache{path: cachePath,
		key: cacheKey{Format: cacheFormat, Path: core.ResolvedPath(indexPath), Stamp: statcache.Of(st)}}
}

func (c *Cache) legacyLoad() *Graph {
	if c.path == "" {
		return nil
	}
	data, err := os.ReadFile(c.path)
	if err != nil {
		return nil
	}
	var cf cacheFile
	if gob.NewDecoder(bytes.NewReader(data)).Decode(&cf) != nil || cf.Key != c.key {
		return nil
	}
	return fromCache(&cf)
}

func legacyWriteGob(path string, v any) bool {
	var buf bytes.Buffer
	if gob.NewEncoder(&buf).Encode(v) != nil {
		return false
	}
	if os.MkdirAll(filepath.Dir(path), 0o700) != nil {
		return false
	}
	return atomicfile.WriteStream(path, 0o600, func(w io.Writer) error {
		_, err := w.Write(buf.Bytes())
		return err
	}) == nil
}

func TestNewCacheMatchesLegacy(t *testing.T) {
	base, path := newCachedBase(t)
	cacheMinBytes = int64(len(fixtureIndex(t)))
	small := filepath.Join(base, "small.json")
	if err := os.WriteFile(small, []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(base, "link.json")
	if err := os.Symlink(path, link); err != nil {
		t.Skipf("no symlink: %v", err)
	}
	for _, p := range []string{path, link, small, base, filepath.Join(base, "missing")} {
		got, want := NewCache(p, "c.gob"), legacyNewCache(p, "c.gob")
		if *got != *want {
			t.Errorf("%s: %+v, legacy %+v", filepath.Base(p), *got, *want)
		}
	}
	if NewCache(path, "c.gob").path == "" {
		t.Error("an index of exactly cacheMinBytes is not cached")
	}
}

func TestLoadMatchesLegacy(t *testing.T) {
	base, path := newCachedBase(t)
	cache := CachePath(base)
	if !NewCache(path, cache).Store(decoded(t, path)) {
		t.Fatal("Store failed")
	}
	data, err := os.ReadFile(cache)
	if err != nil {
		t.Fatal(err)
	}
	stale := NewCache(path, cache)
	stale.key.Size++
	cases := []struct {
		name string
		body []byte // nil keeps the stored cache; the empty slice removes it
		c    *Cache
		hit  bool
	}{
		{"hit", nil, NewCache(path, cache), true},
		{"other key", nil, stale, false},
		{"not cacheable", nil, &Cache{}, false},
		{"missing", []byte{}, NewCache(path, cache), false},
		{"garbage", []byte("garbage"), NewCache(path, cache), false},
		{"truncated", data[:len(data)-3], NewCache(path, cache), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			switch {
			case tc.body == nil:
				if err := os.WriteFile(cache, data, 0o600); err != nil {
					t.Fatal(err)
				}
			case len(tc.body) == 0:
				_ = os.Remove(cache)
			default:
				if err := os.WriteFile(cache, tc.body, 0o600); err != nil {
					t.Fatal(err)
				}
			}
			got, want := tc.c.Load(), tc.c.legacyLoad()
			if (got != nil) != tc.hit || (want != nil) != tc.hit {
				t.Fatalf("hit %v, legacy %v; want %v", got != nil, want != nil, tc.hit)
			}
			if got != nil {
				sameGraph(t, got, want)
			}
		})
	}
}

// TestStoreBytesMatchLegacy: Store writes the bytes the legacy writer wrote
// for the same cache file.
func TestStoreBytesMatchLegacy(t *testing.T) {
	base, path := newCachedBase(t)
	cache := CachePath(base)
	if !NewCache(path, cache).Store(decoded(t, path)) {
		t.Fatal("Store failed")
	}
	got, _ := os.ReadFile(cache)
	var cf cacheFile
	if err := gob.NewDecoder(bytes.NewReader(got)).Decode(&cf); err != nil {
		t.Fatal(err)
	}
	legacy := filepath.Join(t.TempDir(), "legacy.gob")
	if !legacyWriteGob(legacy, &cf) {
		t.Fatal("legacy write failed")
	}
	if want, _ := os.ReadFile(legacy); !bytes.Equal(got, want) {
		t.Fatalf("cache bytes differ from the legacy writer:\n%x\n%x", got, want)
	}
	for _, name := range []string{"cacheFile", "cacheKey", "Callees", "MtimeNs"} {
		if !bytes.Contains(got, []byte(name)) {
			t.Errorf("cache bytes do not name %s", name)
		}
	}
}
