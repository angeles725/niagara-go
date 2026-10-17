package blockfiles

import (
	"errors"
	"os"
	"strings"
	"syscall"
	"unicode/utf8"

	"github.com/angeles725/niagara-go/internal/pycompat"
)

// The text-file readers below are shared by the corpus tools (module-find, corpus-nav,
// check-coverage, gen-catalog). They belong in pycompat (ReadTextReplace was approved
// there) and live here only until that package grows them.

// ErrNotUTF8 is what ReadText returns for a file that is not valid UTF-8 (Python raises
// UnicodeDecodeError); its text is what the tools print for it.
var ErrNotUTF8 = errors.New("not valid UTF-8")

// ReadText is open(path, encoding="utf-8").read(): strict UTF-8 (ErrNotUTF8 otherwise)
// with universal newlines.
func ReadText(path string) (string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	if !utf8.Valid(raw) {
		return "", ErrNotUTF8
	}
	return pycompat.UniversalNewlines(string(raw)), nil
}

// ReadTextReplace is open(path, encoding="utf-8", errors="replace").read(): undecodable
// bytes become U+FFFD as pycompat.DecodeReplace does, with universal newlines.
func ReadTextReplace(path string) (string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return pycompat.UniversalNewlines(pycompat.DecodeReplace(raw)), nil
}

// Strerror is the OSError.strerror Python shows for a failed read ("Is a directory"):
// the C library text of the errno err holds, mapped like pycompat.OSErrorText (whose
// "[Errno n] " prefix it drops). An error without an errno keeps its own text.
func Strerror(err error) string {
	var errno syscall.Errno
	if !errors.As(err, &errno) {
		return err.Error()
	}
	_, text, _ := strings.Cut(pycompat.OSErrorText(errno), "] ")
	return text
}
