package secaudit

// Python text semantics the audit relies on that internal/pycompat does not provide:
// json.dumps(str) and OSError.strerror of a Go error. Everything else comes from pycompat.

import (
	"errors"
	"fmt"
	"io/fs"
	"strings"
	"syscall"
	"unicode"
	"unicode/utf8"
)

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
