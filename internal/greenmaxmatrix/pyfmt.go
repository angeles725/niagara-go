package greenmaxmatrix

// Python text semantics the matrix relies on that internal/pycompat does not provide: the
// Unicode \s and trailing $ of re patterns, str slicing by code point, csv.writer rows and the
// OSError texts. Everything else comes from pycompat.

import (
	"errors"
	"fmt"
	"io/fs"
	"strings"
	"syscall"
	"unicode"
	"unicode/utf8"

	"github.com/angeles725/niagara-go/internal/pycompat"
)

// pyWSChars are the characters of Python's Unicode \s, as RE2 class content.
const pyWSChars = `\t\n\v\f\r \x{1c}-\x{1f}\x{85}\x{a0}\x{1680}\x{2000}-\x{200a}\x{2028}\x{2029}\x{202f}\x{205f}\x{3000}`

// pyRe rewrites every \s of a pattern as Python's Unicode whitespace class and every trailing
// `$` as Python's (end, or before a final newline).
func pyRe(pattern string) string {
	pattern = strings.ReplaceAll(pattern, `\s`, `[`+pyWSChars+`]`)
	if strings.HasSuffix(pattern, "$") {
		pattern = strings.TrimSuffix(pattern, "$") + `\n?$`
	}
	return pattern
}

// head is s[:n] counted in code points.
func head(s string, n int) string {
	i := 0
	for pos := range s {
		if i == n {
			return s[:pos]
		}
		i++
	}
	return s
}

// csvRow renders fields like Python's csv.writer (QUOTE_MINIMAL, "\r\n").
func csvRow(fields []string) string {
	out := make([]string, len(fields))
	for i, f := range fields {
		if strings.ContainsAny(f, ",\"\r\n") {
			f = `"` + strings.ReplaceAll(f, `"`, `""`) + `"`
		}
		out[i] = f
	}
	return strings.Join(out, ",") + "\r\n"
}

// pyOSError renders an OS error like str(OSError): "[Errno 2] No such file or directory: 'p'".
func pyOSError(err error) string {
	var pe *fs.PathError
	var errno syscall.Errno
	if !errors.As(err, &pe) || !errors.As(pe.Err, &errno) {
		return err.Error()
	}
	msg := errno.Error()
	if r, size := utf8.DecodeRuneInString(msg); size > 0 {
		msg = string(unicode.ToUpper(r)) + msg[size:]
	}
	return fmt.Sprintf("[Errno %d] %s: %s", int(errno), msg, pycompat.StrRepr(pe.Path))
}
