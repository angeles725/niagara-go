package classrank

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
	"github.com/angeles725/niagara-go/internal/index"
	"github.com/angeles725/niagara-go/internal/nav/core"
	"github.com/angeles725/niagara-go/internal/statcache"
)

// Equivalence with the code before internal/statcache: the legacy* functions
// are the hand-rolled stat, gob decode and gob write this cache had, verbatim,
// and every case must decide (and write) as they did.

func legacyStatOf(path string) fileStat {
	st, err := os.Stat(path)
	if err != nil {
		return fileStat{}
	}
	// The key is the one change since: the whole statcache.Stamp, ctime and inode too.
	return fileStat{true, statcache.Of(st)}
}

func legacyReadCache(path string, key cacheKey) *Ranker {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var cf cacheFile
	if gob.NewDecoder(bytes.NewReader(data)).Decode(&cf) != nil || cf.Key != key {
		return nil
	}
	var meta *index.Object
	if cf.Meta != nil {
		v, err := index.Decode(cf.Meta)
		if meta, _ = v.(*index.Object); err != nil || meta == nil {
			return nil
		}
	}
	r, err := newRanker(cf.State, meta)
	if err != nil {
		return nil
	}
	return r
}

func legacyWriteCache(path string, key cacheKey, st state, meta *index.Object) bool {
	cf := cacheFile{Key: key, State: st}
	if meta != nil {
		cf.Meta = []byte(core.Dumps(meta, -1))
	}
	var buf bytes.Buffer
	if gob.NewEncoder(&buf).Encode(&cf) != nil {
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

func TestStatOfMatchesLegacy(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "class-index.json")
	if err := os.WriteFile(p, []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	mt := time.Unix(1_700_000_000, 42)
	if err := os.Chtimes(p, mt, mt); err != nil {
		t.Fatal(err)
	}
	paths := []string{p, dir, filepath.Join(dir, "missing")}
	if link := filepath.Join(dir, "link"); os.Symlink(p, link) == nil {
		paths = append(paths, link)
	}
	for _, path := range paths {
		if got, want := statOf(path), legacyStatOf(path); got != want {
			t.Errorf("%s: %+v, legacy %+v", filepath.Base(path), got, want)
		}
	}
	if got := statOf(filepath.Join(dir, "missing")); got != (fileStat{}) {
		t.Errorf("a missing index is %+v, want the zero fileStat", got)
	}
}

// cachedSource is a Source whose ranker Open has cached; it returns the
// source and the cache file's bytes.
func cachedSource(t *testing.T) (Source, []byte) {
	t.Helper()
	old := cacheMinBytes
	cacheMinBytes = 0
	t.Cleanup(func() { cacheMinBytes = old })
	src := SourceOf(newBase(t, map[string][]byte{ClassIndexFile: skipsIndex(t)}))
	if _, err := Open(src); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(src.Cache)
	if err != nil {
		t.Fatal(err)
	}
	return src, data
}

func TestReadCacheMatchesLegacy(t *testing.T) {
	src, data := cachedSource(t)
	key := keyOf(src)
	stale := key
	stale.Class.MtimeNs++
	other := key
	other.K1 = 1.2
	cases := []struct {
		name string
		body []byte // nil keeps the cache Open wrote
		key  cacheKey
		hit  bool
	}{
		{"hit", nil, key, true},
		{"stale index", nil, stale, false},
		{"other parameters", nil, other, false},
		{"empty", []byte{}, key, false},
		{"garbage", []byte("garbage"), key, false},
		{"truncated", data[:len(data)-1], key, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			body := tc.body
			if body == nil {
				body = data
			}
			if err := os.WriteFile(src.Cache, body, 0o600); err != nil {
				t.Fatal(err)
			}
			got, want := readCache(src.Cache, tc.key), legacyReadCache(src.Cache, tc.key)
			if (got != nil) != tc.hit || (want != nil) != tc.hit {
				t.Fatalf("hit %v, legacy %v; want %v", got != nil, want != nil, tc.hit)
			}
			if got != nil && !reflect.DeepEqual(got.Rank("alarm service"), want.Rank("alarm service")) {
				t.Fatal("the cached ranker ranks unlike the legacy read")
			}
		})
	}
	if err := os.Remove(src.Cache); err != nil {
		t.Fatal(err)
	}
	if readCache(src.Cache, key) != nil || legacyReadCache(src.Cache, key) != nil {
		t.Fatal("a missing cache is a hit")
	}
}

// TestWriteCacheBytesMatchLegacy: the cache file is byte for byte the one the
// legacy writer wrote for the same key, state and meta, mode 0600.
func TestWriteCacheBytesMatchLegacy(t *testing.T) {
	src, data := cachedSource(t)
	var cf cacheFile
	if err := gob.NewDecoder(bytes.NewReader(data)).Decode(&cf); err != nil {
		t.Fatal(err)
	}
	var meta *index.Object
	if cf.Meta != nil {
		v, err := index.Decode(cf.Meta)
		if err != nil {
			t.Fatal(err)
		}
		meta = v.(*index.Object)
	}
	dir := t.TempDir()
	for _, m := range []*index.Object{meta, nil} {
		newPath, oldPath := filepath.Join(dir, "n", cacheName), filepath.Join(dir, "o", cacheName)
		if !writeCache(newPath, keyOf(src), cf.State, m) || !legacyWriteCache(oldPath, keyOf(src), cf.State, m) {
			t.Fatal("write failed")
		}
		got, _ := os.ReadFile(newPath)
		if want, _ := os.ReadFile(oldPath); !bytes.Equal(got, want) {
			t.Fatalf("meta %v: cache bytes differ from the legacy writer", m != nil)
		}
		if st, err := os.Stat(newPath); err != nil || st.Mode().Perm() != 0o600 {
			t.Fatalf("cache mode %v, %v", st.Mode(), err)
		}
		for _, name := range []string{"cacheFile", "cacheKey", "fileStat", "Present", "Haystacks"} {
			if !bytes.Contains(got, []byte(name)) {
				t.Errorf("cache bytes do not name %s", name)
			}
		}
	}
	// The file Open wrote is the legacy writer's for its own key, state and meta.
	legacy := filepath.Join(dir, "legacy.gob")
	if !legacyWriteCache(legacy, keyOf(src), cf.State, meta) {
		t.Fatal("legacy write failed")
	}
	if want, _ := os.ReadFile(legacy); !bytes.Equal(data, want) {
		t.Fatal("Open's cache bytes differ from the legacy writer")
	}
}
