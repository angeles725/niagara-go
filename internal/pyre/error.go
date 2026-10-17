package pyre

import (
	"fmt"
	"strings"
)

// Flags are the re module flags.
type Flags int

// The flags of Python's re module (same bit values).
const (
	IGNORECASE Flags = 2
	LOCALE     Flags = 4
	MULTILINE  Flags = 8
	DOTALL     Flags = 16
	UNICODE    Flags = 32
	VERBOSE    Flags = 64
	ASCII      Flags = 256

	typeFlags = ASCII | LOCALE | UNICODE
)

func flagOf(c rune) Flags {
	return map[rune]Flags{'i': IGNORECASE, 'L': LOCALE, 'm': MULTILINE, 's': DOTALL, 'x': VERBOSE,
		'a': ASCII, 'u': UNICODE}[c]
}

// Error is re.error (re.PatternError): Error() is str(exc).
type Error struct {
	Msg     string
	Pattern string
	Pos     int // code-point position in Pattern, -1 when the error has none
}

func newError(msg, pattern string, pos int) *Error {
	return &Error{Msg: msg, Pattern: pattern, Pos: pos}
}

func (e *Error) Error() string {
	if e.Pos < 0 {
		return e.Msg
	}
	msg := fmt.Sprintf("%s at position %d", e.Msg, e.Pos)
	if strings.Contains(e.Pattern, "\n") {
		// lineno = pattern.count("\n", 0, pos) + 1; colno = pos - pattern.rfind("\n", 0, pos)
		rs := []rune(e.Pattern)
		line, last := 1, -1
		for i, r := range rs[:min(e.Pos, len(rs))] {
			if r == '\n' {
				line, last = line+1, i
			}
		}
		msg = fmt.Sprintf("%s (line %d, column %d)", msg, line, e.Pos-last)
	}
	return msg
}
