package bognav

import (
	"bytes"
	"encoding/gob"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/angeles725/niagara-go/internal/atomicfile"
	"github.com/angeles725/niagara-go/internal/statcache"
	"github.com/angeles725/niagara-go/internal/textfold"
)

// Equivalence with the code before internal/statcache (PR #179): the legacy*
// functions are that code verbatim, and every case must decide as it did. The
// legacy key builds the current key shape: the whole statcache.Stamp per file
// (format 2), Missing when the file cannot be stat'ed.

func legacyCatalogCacheDir() string {
	if textfold.Lower(os.Getenv("NIAGARA_CACHE_DIR")) == "off" {
		return ""
	}
	home := os.Getenv("XDG_CACHE_HOME")
	if home == "" {
		h, err := os.UserHomeDir()
		if err != nil || h == "" {
			return ""
		}
		home = filepath.Join(h, ".cache")
	}
	return filepath.Join(home, "niagara-go", "bog-nav")
}

func legacyCatalogKeyOf(src string, paths []string) catalogKey {
	k := catalogKey{Format: catalogCacheFormat, Dir: src, Files: make([]catalogFile, len(paths))}
	for i, p := range paths {
		f := catalogFile{Path: p, Stamp: statcache.Missing}
		if st, err := os.Stat(p); err == nil {
			f.Stamp = statcache.Of(st)
		}
		k.Files[i] = f
	}
	return k
}

func legacyReadCatalogCache(path string, key catalogKey) ([]string, map[string]*classRec, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, false
	}
	var cf catalogCache
	if gob.NewDecoder(bytes.NewReader(data)).Decode(&cf) != nil || !cf.Key.equal(key) {
		return nil, nil, false
	}
	keys := make([]string, 0, len(cf.Entries))
	recs := make(map[string]*classRec, len(cf.Entries))
	for _, e := range cf.Entries {
		if _, dup := recs[e.Key]; dup {
			return nil, nil, false
		}
		recs[e.Key] = &classRec{Type: e.Type, imports: e.Imports, source: e.Source}
		keys = append(keys, e.Key)
	}
	return keys, recs, true
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

// TestCatalogCacheDirMatchesLegacy: the off switch stays NIAGARA_CACHE_DIR=off
// only (no trim, no 0/no/false), and the directory is the one it was.
func TestCatalogCacheDirMatchesLegacy(t *testing.T) {
	for _, sw := range []string{"", "off", "OFF", "Off", " off", "off ", "0", "no", "false", "on"} {
		for _, xdg := range []string{"", "/x/cache"} {
			for _, home := range []string{"", "/home/u"} {
				t.Setenv("NIAGARA_CACHE_DIR", sw)
				t.Setenv("XDG_CACHE_HOME", xdg)
				t.Setenv("HOME", home)
				if got, want := catalogCacheDir(), legacyCatalogCacheDir(); got != want {
					t.Errorf("switch %q xdg %q home %q: %q, legacy %q", sw, xdg, home, got, want)
				}
			}
		}
	}
	t.Setenv("XDG_CACHE_HOME", "/x/cache")
	for sw, on := range map[string]bool{"off": false, "OFF": false, " off": true, "0": true, "false": true} {
		t.Setenv("NIAGARA_CACHE_DIR", sw)
		if (catalogCacheDir() != "") != on {
			t.Errorf("NIAGARA_CACHE_DIR=%q: cache dir %q, want on %v", sw, catalogCacheDir(), on)
		}
	}
}

func TestCatalogKeyMatchesLegacy(t *testing.T) {
	root := miniRoot(t)
	paths := javaFiles(root)
	if len(paths) == 0 {
		t.Fatal("no .java files in the fixture")
	}
	paths = append(paths, filepath.Join(root, "gone.java"), root)
	got, want := catalogKeyOf(root, paths), legacyCatalogKeyOf(root, paths)
	if !reflect.DeepEqual(got, want) || !got.equal(want) {
		t.Fatalf("key %+v\nlegacy %+v", got, want)
	}
	if f := got.Files[len(paths)-2]; f.Stamp != statcache.Missing {
		t.Fatalf("missing file stamp %+v, want statcache.Missing", f)
	}
}

func TestReadCatalogCacheMatchesLegacy(t *testing.T) {
	cache := cacheEnv(t)
	root := miniRoot(t)
	if key, _ := fallbackDefault(t, root); key == "" {
		t.Fatal("BooleanWritable did not resolve")
	}
	files := cacheFiles(t, cache)
	if len(files) == 0 {
		t.Fatal("no catalog cache written")
	}
	data, _ := os.ReadFile(files[0])
	var cf catalogCache
	if err := gob.NewDecoder(bytes.NewReader(data)).Decode(&cf); err != nil {
		t.Fatal(err)
	}
	key := cf.Key
	stale := catalogKey{Format: key.Format, Dir: key.Dir, Files: append([]catalogFile(nil), key.Files...)}
	stale.Files[0].MtimeNs++
	dup := cf
	dup.Entries = append(append([]catalogEntry(nil), cf.Entries...), cf.Entries[0])
	dir := t.TempDir()
	write := func(name string, v any) string {
		p := filepath.Join(dir, name)
		if b, ok := v.([]byte); ok {
			if err := os.WriteFile(p, b, 0o600); err != nil {
				t.Fatal(err)
			}
		} else if !legacyWriteGob(p, v) {
			t.Fatal("legacy write failed")
		}
		return p
	}
	cases := []struct {
		name, path string
		key        catalogKey
		hit        bool
	}{
		{"hit", files[0], key, true},
		{"stale key", files[0], stale, false},
		{"missing", filepath.Join(dir, "missing.gob"), key, false},
		{"garbage", write("garbage.gob", []byte("garbage")), key, false},
		{"truncated", write("trunc.gob", data[:len(data)/2]), key, false},
		{"duplicate entry", write("dup.gob", &dup), key, false},
	}
	for _, tc := range cases {
		keys, recs, ok := readCatalogCache(tc.path, tc.key)
		lkeys, lrecs, lok := legacyReadCatalogCache(tc.path, tc.key)
		if ok != tc.hit || lok != tc.hit || !reflect.DeepEqual(keys, lkeys) || !reflect.DeepEqual(recs, lrecs) {
			t.Errorf("%s: hit %v (%d keys), legacy %v (%d keys); want %v", tc.name, ok, len(keys), lok, len(lkeys), tc.hit)
		}
	}
	// The bytes cachedParseDir wrote are the legacy writer's for the same cache.
	legacy := write("legacy.gob", &cf)
	if want, _ := os.ReadFile(legacy); !bytes.Equal(data, want) {
		t.Fatalf("cache bytes differ from the legacy writer")
	}
	for _, name := range []string{"catalogCache", "catalogKey", "catalogFile", "catalogEntry", "MtimeNs"} {
		if !bytes.Contains(data, []byte(name)) {
			t.Errorf("cache bytes do not name %s", name)
		}
	}
}
