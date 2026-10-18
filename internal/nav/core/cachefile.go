package core

import (
	"bytes"
	"encoding/gob"
	"os"
	"path/filepath"
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

// WriteGob writes the gob encoding of v to path atomically: through a
// temporary file "<base>.*.tmp" in its directory (created with mode 0700;
// the file is 0600) and a rename. It returns false, leaving nothing behind,
// when v cannot be encoded or the file cannot be written.
func WriteGob(path string, v any) bool {
	var buf bytes.Buffer
	if gob.NewEncoder(&buf).Encode(v) != nil {
		return false
	}
	dir := filepath.Dir(path)
	if os.MkdirAll(dir, 0o700) != nil {
		return false
	}
	f, err := os.CreateTemp(dir, filepath.Base(path)+".*.tmp")
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
