package pycompat

import (
	"errors"
	"unicode/utf8"
)

// ErrNotUTF8 is what ReadText returns for a file that is not valid UTF-8 (Python raises
// UnicodeDecodeError); its text is what the corpus tools print for it.
var ErrNotUTF8 = errors.New("not valid UTF-8")

// ReadText is open(path, encoding="utf-8").read(): strict UTF-8 (ErrNotUTF8 otherwise)
// with universal newlines. A directory fails as Python's open() does: EISDIR off Windows,
// EACCES ("Permission denied") from the Windows C runtime (readFile).
func ReadText(path string) (string, error) {
	raw, err := readFile(path)
	if err != nil {
		return "", err
	}
	if !utf8.Valid(raw) {
		return "", ErrNotUTF8
	}
	return UniversalNewlines(string(raw)), nil
}

// ReadTextReplace is open(path, encoding="utf-8", errors="replace").read(): undecodable
// bytes become U+FFFD as DecodeReplace does, with universal newlines.
func ReadTextReplace(path string) (string, error) {
	raw, err := readFile(path)
	if err != nil {
		return "", err
	}
	return UniversalNewlines(DecodeReplace(raw)), nil
}
