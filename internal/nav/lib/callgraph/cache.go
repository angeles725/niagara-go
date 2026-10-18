package callgraph

import (
	"bytes"
	"encoding/gob"
	"os"
	"path/filepath"
	"strings"

	"github.com/angeles725/niagara-go/internal/index"
	"github.com/angeles725/niagara-go/internal/nav/core"
)

// The loaded-graph cache, indexes/.mnav-cache/callgraph.gob: decoding the
// 30 MB callgraph-index.json costs about a second on every callers/callees
// run. The cache keeps the Graph under the index's symlink-resolved path,
// size and mtime (ns): an index whose path, size or mtime differs, or a
// cache that cannot be read or is malformed, is ignored (the JSON is decoded
// and the cache rewritten atomically: a temporary file in the same
// directory, then rename; mode 0600). A cache that cannot be written is
// skipped silently, so the output never depends on it.
//
// The names are stored as one string with end offsets: decoding it is one
// allocation, and every caller and callee is a substring of it.
//
// Indexes under cacheMinBytes are not cached: decoding them costs less than
// the cache round trip, and the small read-only test fixtures stay untouched.

const (
	cacheDir    = ".mnav-cache"
	cacheName   = "callgraph.gob"
	cacheFormat = 1
)

// cacheMinBytes is the index size from which the graph is cached.
var cacheMinBytes int64 = 1 << 20

type cacheKey struct {
	Format        int
	Path          string // the index path, absolute with symlinks resolved
	Size, MtimeNs int64
}

type cacheFile struct {
	Key     cacheKey
	Names   string // every caller, then its callees, concatenated in index order
	Ends    []int  // the end offset in Names of each of those strings
	Callees []int  // the callee count of each caller
	Meta    []byte // the index _meta as JSON; nil when absent
}

// Cache is the cache of one callgraph-index.json, keyed when it was created:
// a graph decoded after the index changed is not stored.
type Cache struct {
	path string // the cache file; "" when the index is not cacheable
	key  cacheKey
}

// CachePath is the cache file of a module-navigator base directory.
func CachePath(baseDir string) string {
	return filepath.Join(core.IndexPath(baseDir, cacheDir), cacheName)
}

// NewCache fingerprints the index at indexPath; cachePath is the cache file
// (CachePath). A missing index or one under cacheMinBytes is never cached.
func NewCache(indexPath, cachePath string) *Cache {
	st, err := os.Stat(indexPath)
	if err != nil || !st.Mode().IsRegular() || st.Size() < cacheMinBytes {
		return &Cache{}
	}
	return &Cache{path: cachePath,
		key: cacheKey{Format: cacheFormat, Path: core.ResolvedPath(indexPath), Size: st.Size(), MtimeNs: st.ModTime().UnixNano()}}
}

// Load is the cached Graph of the index, else nil.
func (c *Cache) Load() *Graph {
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

// fromCache rebuilds the Graph of a decoded cache file, or nil when the file
// is inconsistent.
func fromCache(cf *cacheFile) *Graph {
	g := &Graph{callers: make([]string, len(cf.Callees)), callees: make([][]string, len(cf.Callees))}
	if cf.Meta != nil {
		v, err := index.Decode(cf.Meta)
		if g.meta, _ = v.(*index.Object); err != nil || g.meta == nil {
			return nil
		}
	}
	k, start := 0, 0
	next := func() (string, bool) {
		if k >= len(cf.Ends) || cf.Ends[k] < start || cf.Ends[k] > len(cf.Names) {
			return "", false
		}
		s := cf.Names[start:cf.Ends[k]]
		start = cf.Ends[k]
		k++
		return s, true
	}
	var ok bool
	for i, n := range cf.Callees {
		if g.callers[i], ok = next(); !ok || n < 0 || n > len(cf.Ends) {
			return nil
		}
		list := make([]string, n)
		for j := range list {
			if list[j], ok = next(); !ok {
				return nil
			}
		}
		g.callees[i] = list
	}
	if k != len(cf.Ends) || start != len(cf.Names) {
		return nil
	}
	return g
}

// Store writes g to the cache when the index is cacheable and unchanged
// since NewCache; false, leaving nothing behind, when it cannot.
func (c *Cache) Store(g *Graph) bool {
	if c.path == "" || NewCache(c.key.Path, c.path).key != c.key {
		return false
	}
	cf := cacheFile{Key: c.key, Callees: make([]int, len(g.callers))}
	var names strings.Builder
	add := func(s string) {
		names.WriteString(s)
		cf.Ends = append(cf.Ends, names.Len())
	}
	for i, caller := range g.callers {
		add(caller)
		for _, callee := range g.callees[i] {
			add(callee)
		}
		cf.Callees[i] = len(g.callees[i])
	}
	cf.Names = names.String()
	if g.meta != nil {
		cf.Meta = []byte(core.Dumps(g.meta, -1))
	}
	return core.WriteGob(c.path, &cf)
}
