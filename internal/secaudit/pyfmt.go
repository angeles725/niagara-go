package secaudit

// Python text semantics the audit relies on that internal/pycompat does not provide: the Unicode
// \s of re patterns, str slicing by code point, universal-newline line iteration, json.dumps(str)
// and the OSError texts. Everything else comes from pycompat.

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

// pyWSChars are the characters of Python's Unicode \s (Go's \s is ASCII only), as RE2 class content.
const pyWSChars = `\t\n\v\f\r \x{1c}-\x{1f}\x{85}\x{a0}\x{1680}\x{2000}-\x{200a}\x{2028}\x{2029}\x{202f}\x{205f}\x{3000}`

// pyRe rewrites every \s of a pattern as Python's Unicode whitespace: inside a character class
// (written `[\s` first) and as a class of its own elsewhere.
func pyRe(pattern string) string {
	pattern = strings.ReplaceAll(pattern, `[\s`, `[`+pyWSChars)
	return strings.ReplaceAll(pattern, `\s`, `[`+pyWSChars+`]`)
}

// head is s[:n] counted in code points, as Python slices a str.
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

// splitLines splits text the way iterating a Python text file does with universal newlines:
// \r\n and a lone \r end a line like \n and are read as \n; the last line may lack one.
func splitLines(text string) []string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	var lines []string
	for text != "" {
		i := strings.IndexByte(text, '\n')
		if i < 0 {
			lines = append(lines, text)
			break
		}
		lines = append(lines, text[:i+1])
		text = text[i+1:]
	}
	return lines
}

// pyJSONString is json.dumps(str) with the default ensure_ascii=True.
func pyJSONString(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		switch {
		case r == '"':
			b.WriteString(`\"`)
		case r == '\\':
			b.WriteString(`\\`)
		case r == '\n':
			b.WriteString(`\n`)
		case r == '\r':
			b.WriteString(`\r`)
		case r == '\t':
			b.WriteString(`\t`)
		case r == '\b':
			b.WriteString(`\b`)
		case r == '\f':
			b.WriteString(`\f`)
		case r < 0x20 || r > 0x7e && r < 0x10000:
			fmt.Fprintf(&b, `\u%04x`, r)
		case r >= 0x10000:
			r -= 0x10000
			fmt.Fprintf(&b, `\u%04x\u%04x`, 0xd800+(r>>10), 0xdc00+(r&0x3ff))
		default:
			b.WriteRune(r)
		}
	}
	b.WriteByte('"')
	return b.String()
}

// strerror is the C strerror text Python shows for an OS error ("Permission denied").
func strerror(err error) string {
	var errno syscall.Errno
	if errors.As(err, &errno) {
		msg := errno.Error()
		if r, size := utf8.DecodeRuneInString(msg); size > 0 {
			return string(unicode.ToUpper(r)) + msg[size:]
		}
		return msg
	}
	var pe *fs.PathError
	if errors.As(err, &pe) {
		return pe.Err.Error()
	}
	return err.Error()
}

// pyOSError renders an OS error like str(OSError): "[Errno 2] No such file or directory: 'p'".
func pyOSError(err error) string {
	var pe *fs.PathError
	var errno syscall.Errno
	if errors.As(err, &pe) && errors.As(pe.Err, &errno) {
		return fmt.Sprintf("[Errno %d] %s: %s", int(errno), strerror(errno), pycompat.StrRepr(pe.Path))
	}
	return err.Error()
}
