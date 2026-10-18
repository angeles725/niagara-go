// Package pycsv is Python's csv.writer with the excel dialect, byte for byte: QUOTE_MINIMAL,
// quotechar '"' doubled inside quoted fields, no escapechar, with a configurable delimiter
// and line terminator. Go's encoding/csv differs (it never quotes a field that is the only
// empty one in its row, and it quotes leading spaces).
package pycsv

import (
	"io"
	"strings"

	"github.com/angeles725/niagara-go/internal/pycompat"
)

// Writer writes rows like csv.writer(f, delimiter=Delimiter, lineterminator=LineTerminator).
type Writer struct {
	w io.Writer
	// Delimiter separates the fields (excel: ',').
	Delimiter rune
	// LineTerminator ends every row (excel: "\r\n").
	LineTerminator string
}

// NewWriter returns a Writer with the excel dialect's delimiter and line terminator.
func NewWriter(w io.Writer) *Writer { return &Writer{w: w, Delimiter: ',', LineTerminator: "\r\n"} }

// WriteRow is writer.writerow(fields): nil is the empty field (None), a string is written as
// is and any other value as pycompat.Str (str() for bool, int and float).
func (w *Writer) WriteRow(fields []any) error {
	_, err := io.WriteString(w.w, w.FormatRow(fields))
	return err
}

// FormatRow returns the text WriteRow writes for fields, line terminator included.
func (w *Writer) FormatRow(fields []any) string {
	var b strings.Builder
	for i, v := range fields {
		if i > 0 {
			b.WriteRune(w.Delimiter)
		}
		s := ""
		switch x := v.(type) {
		case nil:
		case string:
			s = x
		default:
			s = pycompat.Str(x)
		}
		// A row that is one empty field is written as "" so it reads back as [''], not [].
		if w.needsQuotes(s) || len(fields) == 1 && s == "" {
			b.WriteByte('"')
			b.WriteString(strings.ReplaceAll(s, `"`, `""`))
			b.WriteByte('"')
		} else {
			b.WriteString(s)
		}
	}
	b.WriteString(w.LineTerminator)
	return b.String()
}

// needsQuotes is QUOTE_MINIMAL: the field holds the delimiter, the quote character, CR, LF
// or a character of the line terminator.
func (w *Writer) needsQuotes(s string) bool {
	return strings.ContainsFunc(s, func(r rune) bool {
		return r == w.Delimiter || r == '"' || r == '\n' || r == '\r' || strings.ContainsRune(w.LineTerminator, r)
	})
}
