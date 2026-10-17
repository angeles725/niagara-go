package classrank

import (
	"bytes"
	"encoding/gob"
	"os"
	"path/filepath"

	"github.com/angeles725/niagara-go/internal/index"
	"github.com/angeles725/niagara-go/internal/nav/core"
	"github.com/angeles725/niagara-go/internal/nav/registry"
)

// The prepared-ranker cache (class_rank_cache.py), Go's own file: Python's
// indexes/class-rank.cache is a marshal dump this port neither reads nor
// overwrites. The Go cache is indexes/.mnav-cache/class-rank.gob, a gob of
// (key, state, meta) where the key fingerprints what the state depends on:
//
//	cacheFormat (bump it when state or the term rules change),
//	the ranker parameters,
//	size and mtime (ns) of class-index.json and of xref-index.json.
//
// A cache whose key differs, that cannot be read or is malformed is ignored:
// the ranker is built and the cache rewritten atomically (a temporary file in
// the same directory, then rename; mode 0600). A cache that cannot be written
// is skipped silently. stderr is the same on a hit and on a build (announce).
//
// Class indexes under cacheMinBytes are not cached: building their ranker
// costs less than the cache round trip, and the small read-only test fixtures
// stay untouched.

const (
	cacheDir    = ".mnav-cache"
	cacheName   = "class-rank.gob"
	cacheFormat = 1
)

// cacheMinBytes is the class index size from which the ranker is cached.
var cacheMinBytes int64 = 1 << 20

// fileStat is (size, mtime_ns) of one index; Present false when it is absent.
type fileStat struct {
	Present bool
	Size    int64
	MtimeNs int64
}

type cacheKey struct {
	Format             int
	K1, B, Title, Prio float64
	Class, Xref        fileStat
}

type cacheFile struct {
	Key   cacheKey
	State state
	Meta  []byte // the class index _meta as JSON; nil when absent
}

func statOf(path string) fileStat {
	st, err := os.Stat(path)
	if err != nil {
		return fileStat{}
	}
	return fileStat{true, st.Size(), st.ModTime().UnixNano()}
}

func keyOf(env *registry.Env) cacheKey {
	return cacheKey{Format: cacheFormat, K1: 1.5, B: 0.3, Title: titleWeight, Prio: priorWeight,
		Class: statOf(absIndex(env, classIndex)), Xref: statOf(absIndex(env, xrefIndex))}
}

func cachePath(env *registry.Env) string {
	return filepath.Join(core.IndexPath(env.BaseDir, cacheDir), cacheName)
}

// readCache is class_rank_cache.read plus ClassRanker.from_state: the ranker
// stored under key, else nil.
func readCache(env *registry.Env, key cacheKey) *Ranker {
	data, err := os.ReadFile(cachePath(env))
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

// writeCache is class_rank_cache.write; false, leaving nothing behind, when
// the cache cannot be written.
func writeCache(env *registry.Env, key cacheKey, st state, meta *index.Object) bool {
	cf := cacheFile{Key: key, State: st}
	if meta != nil {
		cf.Meta = []byte(core.Dumps(meta, -1))
	}
	var buf bytes.Buffer
	if gob.NewEncoder(&buf).Encode(&cf) != nil {
		return false
	}
	path := cachePath(env)
	if os.MkdirAll(filepath.Dir(path), 0o700) != nil {
		return false
	}
	f, err := os.CreateTemp(filepath.Dir(path), cacheName+".*.tmp")
	if err != nil {
		return false
	}
	_, werr := f.Write(buf.Bytes())
	if cerr := f.Close(); werr == nil {
		werr = cerr
	}
	if werr == nil {
		werr = os.Rename(f.Name(), path)
	}
	if werr != nil {
		os.Remove(f.Name())
		return false
	}
	return true
}

// announce is class_rank_cache.announce: on a cache hit, the stderr a build
// prints (progress line, staleness warnings, the absent-xref NOTE or the xref
// progress line).
func announce(env *registry.Env, meta *index.Object) {
	path := absIndex(env, classIndex)
	progressEnd(env, progressStart(env, path, classIndex), true)
	warnStale(env, meta, path)
	if !xrefAbsent(env) {
		progressEnd(env, progressStart(env, absIndex(env, xrefIndex), xrefIndex), true)
	}
}

// Load is load_ranker: the Ranker of the base directory's indexes, or nil
// after an exit-2 error: the cached one, else a full build (which then
// refreshes the cache unless an index changed while it was read).
func Load(env *registry.Env) *Ranker {
	key := keyOf(env)
	cached := key.Class.Size >= cacheMinBytes
	if cached {
		if r := readCache(env, key); r != nil {
			announce(env, r.meta)
			return r
		}
	}
	classes, importedBy, meta, ok := readInputs(env)
	if !ok {
		return nil
	}
	st := build(classes, importedBy)
	r, err := newRanker(st, meta)
	if err != nil { // a freshly built state is always well formed
		panic(err)
	}
	if cached && key == keyOf(env) {
		writeCache(env, key, st, meta)
	}
	return r
}
