// Package statcache is what niagara-go's stat-keyed caches share: the stat
// stamp of a file or a file set, the atomic gob file, the off switch, the user
// cache directory and the test-binary check. Each cache keeps its own key
// type, file name, format version and invalidation rule (docs/statcache.md
// lists them); this package only removes the copies of the common steps.
package statcache

import (
	"bytes"
	"encoding/gob"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/angeles725/niagara-go/internal/atomicfile"
	"github.com/angeles725/niagara-go/internal/pycompat"
)

// Stamp is the size and modification time (Unix ns) of a file: what most
// caches key on.
type Stamp struct{ Size, MtimeNs int64 }

// Missing is the Stamp of a path that cannot be stat'ed.
var Missing = Stamp{-1, -1}

// Exists reports whether s is the stamp of a file that could be stat'ed.
func (s Stamp) Exists() bool { return s != Missing }

// Of is the Stamp of fi.
func Of(fi fs.FileInfo) Stamp { return Stamp{fi.Size(), fi.ModTime().UnixNano()} }

// Stat is the Stamp of path (os.Stat: a link is its target), Missing when it
// cannot be stat'ed.
func Stat(path string) Stamp {
	st, err := os.Stat(path)
	if err != nil {
		return Missing
	}
	return Of(st)
}

// Regular is the Stamp of path when it is a regular file of at least minSize
// bytes; ok is false otherwise (such a file is not cached).
func Regular(path string, minSize int64) (s Stamp, ok bool) {
	st, err := os.Stat(path)
	if err != nil || !st.Mode().IsRegular() || st.Size() < minSize {
		return Stamp{}, false
	}
	return Of(st), true
}

// Same reports whether path still has stamp s. A result computed from a file
// that changed meanwhile is not stored: callers stat before the work and ask
// Same after it.
func Same(path string, s Stamp) bool { return Stat(path) == s && s.Exists() }

// Files is the Stamp of every path, in order (Missing for one that cannot be
// stat'ed); workers > 1 stats the paths in that many contiguous chunks at once.
func Files(paths []string, workers int) []Stamp {
	out := make([]Stamp, len(paths))
	if workers < 1 {
		workers = 1
	}
	chunk := (len(paths) + workers - 1) / workers
	var wg sync.WaitGroup
	for lo := 0; lo < len(paths); lo += chunk {
		hi := min(lo+chunk, len(paths))
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := lo; i < hi; i++ {
				out[i] = Stat(paths[i])
			}
		}()
	}
	wg.Wait()
	return out
}

// ReadGob decodes the gob file at path into v; false when it cannot be read
// or does not decode. It checks no key or format: the caller compares its own
// in v.
func ReadGob(path string, v any) bool {
	data, err := os.ReadFile(path)
	return err == nil && gob.NewDecoder(bytes.NewReader(data)).Decode(v) == nil
}

// WriteGob writes the gob encoding of v to path atomically: v is encoded in
// memory, path's directory is created with mode 0700 when missing, then
// atomicfile.WriteStream writes a temp file beside path, sets it to 0600 and
// renames it over path. It returns false, leaving nothing behind, when v
// cannot be encoded or the file cannot be written.
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

// Off reports whether the environment variable env turns a cache off: off,
// 0, no or false, in any case and with surrounding space.
func Off(env string) bool {
	switch strings.ToLower(pycompat.Strip(os.Getenv(env))) {
	case "off", "0", "no", "false":
		return true
	}
	return false
}

// UserDir is $XDG_CACHE_HOME (else ~/.cache)/app/name, "" when neither is
// known.
func UserDir(app, name string) string {
	home := os.Getenv("XDG_CACHE_HOME")
	if home == "" {
		h, err := os.UserHomeDir()
		if err != nil || h == "" {
			return ""
		}
		home = filepath.Join(h, ".cache")
	}
	return filepath.Join(home, app, name)
}

// TestBinary reports whether this executable is a go test binary (or cannot
// be found): such a run never writes a user cache.
func TestBinary() bool {
	exe, err := os.Executable()
	return err != nil || IsTestName(exe)
}

// IsTestName reports whether exe names a go test binary (foo.test, foo.test.exe).
func IsTestName(exe string) bool {
	return strings.HasSuffix(strings.TrimSuffix(filepath.Base(exe), ".exe"), ".test")
}
