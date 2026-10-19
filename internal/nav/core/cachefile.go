package core

import (
	"bytes"
	"encoding/gob"
	"io"
	"os"
	"path/filepath"

	"github.com/angeles725/niagara-go/internal/atomicfile"
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

// WriteGob writes the gob encoding of v to path atomically
// (atomicfile.WriteStream: a temp file in its directory, created with mode
// 0700, and a rename; the file is 0600). It returns false, leaving nothing
// behind, when v cannot be encoded or the file cannot be written.
func WriteGob(path string, v any) bool {
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
