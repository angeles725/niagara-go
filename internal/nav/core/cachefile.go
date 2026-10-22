package core

import (
	"path/filepath"

	"github.com/angeles725/niagara-go/internal/statcache"
)

// The Go-only caches of module-nav (the inventory stats, the call graph and
// the class ranker under indexes/.mnav-cache) share their key path and their
// writer.

// ResolvedPath is the absolute path with symlinks resolved, the path itself
// (made absolute) when it cannot be resolved: a cache key, so a base
// directory reached through a link shares the entries of the real one.
// Unlike realpath (os.path.realpath) it does not resolve the parents of a
// path that does not exist.
func ResolvedPath(p string) string {
	p = absPath(p)
	if real, err := filepath.EvalSymlinks(p); err == nil {
		return real
	}
	return p
}

// WriteGob is statcache.WriteGob: the gob encoding of v written to path
// atomically (mode 0600), false, leaving nothing behind, when it cannot be.
func WriteGob(path string, v any) bool { return statcache.WriteGob(path, v) }
