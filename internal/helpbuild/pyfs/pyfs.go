package pyfs

import (
	"errors"
	"io"
	"io/fs"
	"os"
	"syscall"

	"github.com/angeles725/niagara-go/internal/index"
	"github.com/angeles725/niagara-go/internal/pycompat"
	"github.com/angeles725/niagara-go/internal/pyjson"
)

// File-system helpers with the semantics of the Python calls the S3/S4
// builders make: os.walk, os.remove, shutil.copyfile and json.load.

// Walk is os.walk(top) top-down without followlinks. visit receives each
// directory (joined like os.path.join, from top), the names of its
// subdirectories (a symlink to a directory counts as one, as
// DirEntry.is_dir() follows links) and of everything else, and returns the
// subdirectories to descend, like a caller editing dirnames in place. A
// symlinked subdirectory is listed but not descended. A directory that
// cannot be read is skipped silently (onerror=None).
func Walk(top string, visit func(dir string, dirs, files []string) []string) {
	walkPy(top, os.ReadDir, nil, visit)
}

// WalkFn is Walk with os.walk's onerror: it receives each directory
// that cannot be read (as joined from top) and the error, then the walk
// goes on without it.
func WalkFn(top string, onerror func(dir string, err error), visit func(dir string, dirs, files []string) []string) {
	walkPy(top, os.ReadDir, onerror, visit)
}

// WalkDirOrder is Walk with each directory listed in the order the
// file system returns it (os.scandir's order, not sorted), for the
// builders that do not sort what os.walk yields (source_indexer's files,
// slots_indexer's folders and files): their output order follows it.
func WalkDirOrder(top string, visit func(dir string, dirs, files []string) []string) {
	walkPy(top, readDirUnsorted, nil, visit)
}

// readDirUnsorted is os.ReadDir without the sort.
func readDirUnsorted(dir string) ([]fs.DirEntry, error) {
	f, err := os.Open(dir)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return f.ReadDir(-1)
}

func walkPy(top string, list func(string) ([]fs.DirEntry, error), onerror func(dir string, err error),
	visit func(dir string, dirs, files []string) []string) {
	ents, err := list(top)
	if err != nil {
		if onerror != nil {
			onerror(top, err)
		}
		return
	}
	var dirs, files []string
	for _, e := range ents {
		isDirEntry := e.IsDir()
		if e.Type()&fs.ModeSymlink != 0 {
			st, err := os.Stat(pycompat.Join(top, e.Name()))
			isDirEntry = err == nil && st.IsDir()
		}
		if isDirEntry {
			dirs = append(dirs, e.Name())
		} else {
			files = append(files, e.Name())
		}
	}
	for _, d := range visit(top, dirs, files) {
		sub := pycompat.Join(top, d)
		if st, err := os.Lstat(sub); err == nil && st.Mode()&fs.ModeSymlink != 0 {
			continue
		}
		walkPy(sub, list, onerror, visit)
	}
}

// RemoveFile is os.remove: it unlinks a file or a symlink and refuses a
// directory (IsADirectoryError), where os.Remove would delete an empty one.
func RemoveFile(path string) error {
	if st, err := os.Lstat(path); err == nil && st.IsDir() {
		return &fs.PathError{Op: "remove", Path: path, Err: syscall.EISDIR}
	}
	return os.Remove(path)
}

// Copyfile is shutil.copyfile(src, dst): the content only, following
// symlinks, dst created or truncated with the default mode (0666 less the
// umask). The error names the file that failed, as Python's OSError does.
// Before dst is opened (and truncated) it refuses, like Python, two names
// of one file (SameFileError, os.path.samefile: links followed), a named
// pipe (SpecialFileError) and a directory source (IsADirectoryError, which
// Python's open(src, 'rb') raises).
func Copyfile(src, dst string) error {
	if s, err := os.Stat(src); err == nil {
		if d, err := os.Stat(dst); err == nil && os.SameFile(s, d) {
			return &sameFileError{src, dst}
		}
	}
	for _, fn := range []string{src, dst} {
		if st, err := os.Stat(fn); err == nil && st.Mode()&fs.ModeNamedPipe != 0 {
			return &specialFileError{fn} // never opened: a FIFO would block
		}
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	if st, err := in.Stat(); err == nil && st.IsDir() {
		return &fs.PathError{Op: "open", Path: src, Err: syscall.EISDIR}
	}
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o666)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		return err
	}
	return out.Close()
}

// sameFileError is shutil.SameFileError, an OSError: str() quotes both
// names with repr.
type sameFileError struct{ src, dst string }

func (e *sameFileError) Error() string {
	return pycompat.FSRepr(e.src) + " and " + pycompat.FSRepr(e.dst) + " are the same file"
}

// specialFileError is shutil.SpecialFileError, an OSError.
type specialFileError struct{ path string }

func (e *specialFileError) Error() string { return "`" + e.path + "` is a named pipe" }

// JSONLoadError is an error json.load(open(path, encoding="utf-8")) raises
// that the builders catch: an OSError (OS true) or a ValueError (a
// UnicodeDecodeError, a JSONDecodeError or the int digit limit). Msg is
// what str(exception) prints. Other failures (Python's RecursionError) are
// returned as they are.
type JSONLoadError struct {
	OS  bool
	Msg string
}

func (e *JSONLoadError) Error() string { return e.Msg }

// LoadJSON is json.load(open(path, encoding="utf-8")): strict UTF-8 over
// the whole file, universal newlines, then Python's decoder.
func LoadJSON(path string) (pyjson.Value, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		var pe *fs.PathError
		if errors.As(err, &pe) {
			return pyjson.Value{}, &JSONLoadError{OS: true, Msg: index.OSError(pe.Err, path)}
		}
		return pyjson.Value{}, err
	}
	if err := index.CheckUTF8(raw); err != nil {
		return pyjson.Value{}, &JSONLoadError{Msg: err.Error()}
	}
	v, err := pyjson.Loads([]rune(pycompat.UniversalNewlines(string(raw))))
	var rec *pyjson.RecursionError
	if err != nil && !errors.As(err, &rec) {
		return pyjson.Value{}, &JSONLoadError{Msg: err.Error()}
	}
	return v, err
}
