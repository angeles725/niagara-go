package index

import (
	"errors"
	"io/fs"
	"os"
	"syscall"

	"github.com/angeles725/niagara-go/internal/pycompat"
)

// UTF8Error is Python's UnicodeDecodeError for bytes.decode("utf-8").
type UTF8Error = pycompat.UTF8Error

// CheckUTF8 returns nil when data is valid UTF-8, else the first error (a *UTF8Error) as
// CPython's UTF-8 decoder reports it.
func CheckUTF8(data []byte) error {
	if e := pycompat.FindUTF8Error(data); e != nil {
		return e
	}
	return nil
}

// OSError formats a file error as Python's str(OSError) for path: "[Errno 13]
// Permission denied: '<path>'", as pycompat.OSErrorText renders it. Errors without an
// errno keep their Go text.
func OSError(err error, path string) string {
	var errno syscall.Errno
	if !errors.As(err, &errno) {
		return err.Error()
	}
	return pycompat.OSErrorText(&fs.PathError{Path: path, Err: errno})
}

// LoadPython reads a JSON index the way `json.load(open(path, encoding="utf-8"))`
// does and returns the decoded value. An error's text is what Python's
// exception prints: an OSError, a UnicodeDecodeError or a JSONDecodeError.
func LoadPython(path string) (any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		var pe *fs.PathError
		if errors.As(err, &pe) {
			return nil, errors.New(OSError(pe.Err, path))
		}
		return nil, err
	}
	if err := CheckUTF8(data); err != nil {
		return nil, err
	}
	v, err := Decode(data)
	var de *DecodeError
	if errors.As(err, &de) {
		return nil, newDecodeError(data, de.Msg, de.Offset, true)
	}
	return v, err
}
