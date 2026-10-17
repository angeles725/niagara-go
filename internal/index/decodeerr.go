package index

import (
	"fmt"
	"unicode/utf8"
)

// DecodeError is Python's json.JSONDecodeError: Error() is str(e), the message
// with the line, column and character position counted the way Python counts
// them in the text it decoded (code points, after universal-newline
// translation of the file).
type DecodeError struct {
	Msg    string
	Offset int // byte offset in the decoded data
	Line   int
	Col    int
	Pos    int // code point index (Python's e.pos)
}

func (e *DecodeError) Error() string {
	return fmt.Sprintf("%s: line %d column %d (char %d)", e.Msg, e.Line, e.Col, e.Pos)
}

// newDecodeError locates byte offset off of data as json.loads does on the
// decoded text: positions count code points, lines count "\n". With newlines
// set, data is first read the way open() in text mode reads a file: "\r\n" and
// a lone "\r" are one "\n" (universal newlines).
func newDecodeError(data []byte, msg string, off int, newlines bool) *DecodeError {
	pos, lastNL, line := 0, -1, 1
	for i := 0; i < off && i < len(data); {
		c := data[i]
		switch {
		case newlines && c == '\r' && i+1 < len(data) && data[i+1] == '\n':
			lastNL, line = pos, line+1
			i += 2
		case c == '\n' || (newlines && c == '\r'):
			lastNL, line = pos, line+1
			i++
		case c < utf8.RuneSelf:
			i++
		default:
			_, n := utf8.DecodeRune(data[i:])
			i += n
		}
		pos++
	}
	col := pos + 1
	if lastNL >= 0 {
		col = pos - lastNL
	}
	return &DecodeError{Msg: msg, Offset: off, Line: line, Col: col, Pos: pos}
}
