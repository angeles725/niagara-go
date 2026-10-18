package pycompat

import (
	"fmt"
	"unicode/utf8"
)

// UTF8Error is the UnicodeDecodeError CPython's strict UTF-8 decoder raises for
// bytes.decode("utf-8"); Error() is its str().
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

// FindUTF8Error returns the first undecodable sequence in data as CPython reports it, or nil
// when data is valid UTF-8.
func FindUTF8Error(data []byte) *UTF8Error {
	if utf8.Valid(data) {
		return nil
	}
	for i := 0; i < len(data); {
		if data[i] < utf8.RuneSelf {
			i++
			continue
		}
		if r, n := utf8.DecodeRune(data[i:]); r != utf8.RuneError || n > 1 {
			i += n
			continue
		}
		return utf8ErrorAt(data, i)
	}
	return nil
}

// utf8ErrorAt describes the invalid sequence starting at i. CPython names the valid prefix
// before the offending byte, so the range ends at the first byte that breaks the sequence.
func utf8ErrorAt(data []byte, i int) *UTF8Error {
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
	for k := 0; k < need; k, j = k+1, j+1 {
		if j >= len(data) {
			return &UTF8Error{Start: i, End: len(data), Byte: c, Reason: "unexpected end of data"}
		}
		if b := data[j]; (k == 0 && (b < lo || b > hi)) || (k > 0 && (b < 0x80 || b > 0xBF)) {
			return &UTF8Error{Start: i, End: j, Byte: c, Reason: "invalid continuation byte"}
		}
	}
	return nil // unreachable: utf8.DecodeRune rejected the sequence
}
