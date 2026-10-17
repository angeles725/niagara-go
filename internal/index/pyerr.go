package index

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"syscall"
	"unicode/utf8"

	"github.com/angeles725/niagara-go/internal/pycompat"
)

// UTF8Error is Python's UnicodeDecodeError for bytes.decode("utf-8").
type UTF8Error struct {
	Start, End int // the undecodable byte range [Start, End)
	Byte       byte
	Reason     string // "invalid start byte", "invalid continuation byte", "unexpected end of data"
}

func (e *UTF8Error) Error() string {
	if e.End-e.Start == 1 {
		return fmt.Sprintf("'utf-8' codec can't decode byte 0x%02x in position %d: %s", e.Byte, e.Start, e.Reason)
	}
	return fmt.Sprintf("'utf-8' codec can't decode bytes in position %d-%d: %s", e.Start, e.End-1, e.Reason)
}

// CheckUTF8 returns nil when data is valid UTF-8, else the first error as
// CPython's UTF-8 decoder reports it.
func CheckUTF8(data []byte) error {
	if utf8.Valid(data) {
		return nil
	}
	for i := 0; i < len(data); {
		c := data[i]
		if c < utf8.RuneSelf {
			i++
			continue
		}
		if utf8.FullRune(data[i:]) {
			if r, n := utf8.DecodeRune(data[i:]); r != utf8.RuneError || n > 1 {
				i += n
				continue
			}
		}
		return utf8Error(data, i)
	}
	return nil
}

// utf8Error describes the invalid sequence starting at i.
func utf8Error(data []byte, i int) error {
	c := data[i]
	var need int
	lo, hi := byte(0x80), byte(0xBF) // allowed range of the first continuation byte
	switch {
	case c >= 0xC2 && c <= 0xDF:
		need = 1
	case c >= 0xE0 && c <= 0xEF:
		need = 2
		if c == 0xE0 {
			lo = 0xA0
		} else if c == 0xED {
			hi = 0x9F
		}
	case c >= 0xF0 && c <= 0xF4:
		need = 3
		if c == 0xF0 {
			lo = 0x90
		} else if c == 0xF4 {
			hi = 0x8F
		}
	default:
		return &UTF8Error{Start: i, End: i + 1, Byte: c, Reason: "invalid start byte"}
	}
	j := i + 1
	for k := 0; k < need; k++ {
		if j >= len(data) {
			return &UTF8Error{Start: i, End: len(data), Byte: c, Reason: "unexpected end of data"}
		}
		b := data[j]
		if (k == 0 && (b < lo || b > hi)) || (k > 0 && (b < 0x80 || b > 0xBF)) {
			return &UTF8Error{Start: i, End: j, Byte: c, Reason: "invalid continuation byte"}
		}
		j++
	}
	return nil // unreachable for data rejected by utf8.DecodeRune
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
