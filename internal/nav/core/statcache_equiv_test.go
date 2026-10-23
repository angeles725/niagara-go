package core

import (
	"bytes"
	"encoding/gob"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/angeles725/niagara-go/internal/atomicfile"
	"github.com/angeles725/niagara-go/internal/index"
	"github.com/angeles725/niagara-go/internal/statcache"
)

// Equivalence with the code before internal/statcache (PR #179): the legacy*
// functions are that code verbatim, and every case must decide as it did.

func legacyClassCacheKeyOf(path string) (classCacheKey, bool) {
	st, err := os.Stat(path)
	if err != nil || !st.Mode().IsRegular() || st.Size() < classCacheMinBytes {
		return classCacheKey{}, false
	}
	// The key is the one change since: the whole statcache.Stamp, ctime and inode too.
	return classCacheKey{Format: classCacheFormat, Path: ResolvedPath(path), Stamp: statcache.Of(st)}, true
}

func legacyReadClassCache(path string, key classCacheKey) *index.Object {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var cf classCacheFile
	if gob.NewDecoder(bytes.NewReader(data)).Decode(&cf) != nil || cf.Key != key {
		return nil
	}
	o, _ := fromClassCache(&cf).(*index.Object)
	return o
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

func TestClassCacheKeyMatchesLegacy(t *testing.T) {
	old := classCacheMinBytes
	classCacheMinBytes = 4
	t.Cleanup(func() { classCacheMinBytes = old })
	dir := t.TempDir()
	mk := func(name, text string) string {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
		mt := time.Unix(1_700_000_000, 123_456_789)
		if err := os.Chtimes(p, mt, mt); err != nil {
			t.Fatal(err)
		}
		return p
	}
	big, small, edge := mk("big", "123456"), mk("small", "123"), mk("edge", "1234")
	link := filepath.Join(dir, "link")
	if err := os.Symlink(big, link); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{big, small, edge, link, dir, filepath.Join(dir, "missing")} {
		k, ok := classCacheKeyOf(p)
		lk, lok := legacyClassCacheKeyOf(p)
		if k != lk || ok != lok {
			t.Errorf("%s: key %+v, %v; legacy %+v, %v", filepath.Base(p), k, ok, lk, lok)
		}
	}
	if _, ok := classCacheKeyOf(edge); !ok {
		t.Error("a file of exactly classCacheMinBytes is not cached")
	}
}

func TestReadClassCacheMatchesLegacy(t *testing.T) {
	dir := t.TempDir()
	key := classCacheKey{Format: classCacheFormat, Path: "/x/class-index.json", Stamp: statcache.Stamp{Size: 10, MtimeNs: 20}}
	o, err := index.Decode([]byte(`{"_meta": {"n": 1}, "classes": {"BA": [{"kind": "class"}]}}`))
	if err != nil {
		t.Fatal(err)
	}
	good := filepath.Join(dir, "good.gob")
	if !writeClassCache(good, key, o.(*index.Object)) {
		t.Fatal("writeClassCache failed")
	}
	data, _ := os.ReadFile(good)
	files := map[string][]byte{"empty": {}, "garbage": []byte("not a gob"), "truncated": data[:len(data)/2]}
	for name, b := range files {
		if err := os.WriteFile(filepath.Join(dir, name), b, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	other := key
	other.MtimeNs++
	cases := []struct {
		file string
		key  classCacheKey
		hit  bool
	}{
		{"good.gob", key, true}, {"good.gob", other, false}, {"missing", key, false},
		{"empty", key, false}, {"garbage", key, false}, {"truncated", key, false}, {".", key, false},
	}
	for _, c := range cases {
		p := filepath.Join(dir, c.file)
		got, want := readClassCache(p, c.key), legacyReadClassCache(p, c.key)
		if (got != nil) != c.hit || (want != nil) != c.hit {
			t.Fatalf("%s: hit %v, legacy %v; want %v", c.file, got != nil, want != nil, c.hit)
		}
		if got != nil && Dumps(got, -1) != Dumps(want, -1) {
			t.Fatalf("%s: decoded %s, legacy %s", c.file, Dumps(got, -1), Dumps(want, -1))
		}
	}
}

var goldenClassCache = classCacheFile{
	Key:     classCacheKey{Format: classCacheFormat, Path: "/i/class-index.json", Stamp: statcache.Stamp{Size: 7, MtimeNs: 9}},
	Strings: "ab", Ends: []int{1, 2}, Ops: []int{opObject | 1<<opBits, 0, opString | 1<<opBits},
}

func TestClassCacheBytesMatchLegacy(t *testing.T) {
	dir := t.TempDir()
	newPath, oldPath := filepath.Join(dir, "n", "c.gob"), filepath.Join(dir, "o", "c.gob")
	if !WriteGob(newPath, &goldenClassCache) || !legacyWriteGob(oldPath, &goldenClassCache) {
		t.Fatal("write failed")
	}
	got, _ := os.ReadFile(newPath)
	want, _ := os.ReadFile(oldPath)
	if !bytes.Equal(got, want) {
		t.Fatalf("cache bytes differ from the legacy writer:\n%x\n%x", got, want)
	}
	// gob numbers types per process (a hex golden would depend on which tests ran
	// first) but writes their names: a renamed type or field changes the bytes.
	for _, name := range []string{"classCacheFile", "classCacheKey", "Strings", "MtimeNs"} {
		if !bytes.Contains(got, []byte(name)) {
			t.Errorf("cache bytes do not name %s", name)
		}
	}
	for _, p := range []string{newPath, oldPath} {
		st, err := os.Stat(p)
		if err != nil || st.Mode().Perm() != 0o600 {
			t.Fatalf("%s mode %v, %v", p, st.Mode(), err)
		}
	}
}
