package secaudit

// Python text semantics the audit relies on. decodeReplace and pyStrRepr are copies of the
// obixnav helpers (a tool never imports another tool's package); they belong in a shared
// internal package once one exists.

import (
	"errors"
	"fmt"
	"io/fs"
	"strings"
	"syscall"
	"unicode"
	"unicode/utf8"
)

// pyWSChars are the characters of Python's Unicode \s (Go's \s is ASCII only), as RE2 class content.
const pyWSChars = `\t\n\v\f\r \x{1c}-\x{1f}\x{85}\x{a0}\x{1680}\x{2000}-\x{200a}\x{2028}\x{2029}\x{202f}\x{205f}\x{3000}`

// pyRe rewrites every \s of a pattern as Python's Unicode whitespace: inside a character class
// (written `[\s` first) and as a class of its own elsewhere.
func pyRe(pattern string) string {
	pattern = strings.ReplaceAll(pattern, `[\s`, `[`+pyWSChars)
	return strings.ReplaceAll(pattern, `\s`, `[`+pyWSChars+`]`)
}

// isPySpace is Python str.isspace: Go's unicode.IsSpace plus the four ASCII separators 0x1c-0x1f.
func isPySpace(r rune) bool { return unicode.IsSpace(r) || r >= 0x1c && r <= 0x1f }

func pyStrip(s string) string { return strings.TrimFunc(s, isPySpace) }

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

// decodeReplace is Python bytes.decode("utf-8", "replace"): each maximal invalid subpart of a
// sequence becomes one U+FFFD.
func decodeReplace(b []byte) string {
	var sb strings.Builder
	for i := 0; i < len(b); {
		c := b[i]
		if c < 0x80 {
			sb.WriteByte(c)
			i++
			continue
		}
		need, lo, hi := 0, byte(0x80), byte(0xbf)
		switch {
		case c >= 0xc2 && c <= 0xdf:
			need = 1
		case c == 0xe0:
			need, lo = 2, 0xa0
		case c >= 0xe1 && c <= 0xef && c != 0xed:
			need = 2
		case c == 0xed:
			need, hi = 2, 0x9f
		case c == 0xf0:
			need, lo = 3, 0x90
		case c >= 0xf1 && c <= 0xf3:
			need = 3
		case c == 0xf4:
			need, hi = 3, 0x8f
		}
		j := i + 1
		for k := 0; k < need; k++ {
			if j >= len(b) || b[j] < lo || b[j] > hi {
				break
			}
			j++
			lo, hi = 0x80, 0xbf
		}
		if need > 0 && j == i+1+need {
			sb.Write(b[i:j])
		} else {
			sb.WriteRune(utf8.RuneError)
		}
		i = j
	}
	return sb.String()
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

// pyJoin is os.path.join(a, b) for a relative b: no cleaning, one '/' between the parts.
func pyJoin(a, b string) string {
	if a == "" || strings.HasSuffix(a, "/") {
		return a + b
	}
	return a + "/" + b
}

// pyStrRepr is Python repr(str): single quotes unless the text has a single quote and no double
// quote; backslash, the quote, \t \n \r and non-printable characters escaped.
func pyStrRepr(s string) string {
	q := byte('\'')
	if strings.Contains(s, "'") && !strings.Contains(s, `"`) {
		q = '"'
	}
	var b strings.Builder
	b.WriteByte(q)
	for _, r := range s {
		switch {
		case r == '\\' || r == rune(q):
			b.WriteByte('\\')
			b.WriteRune(r)
		case r == '\t':
			b.WriteString(`\t`)
		case r == '\n':
			b.WriteString(`\n`)
		case r == '\r':
			b.WriteString(`\r`)
		case unicode.IsPrint(r) || r == ' ':
			b.WriteRune(r)
		case r < 0x100:
			fmt.Fprintf(&b, `\x%02x`, r)
		case r < 0x10000:
			fmt.Fprintf(&b, `\u%04x`, r)
		default:
			fmt.Fprintf(&b, `\U%08x`, r)
		}
	}
	b.WriteByte(q)
	return b.String()
}

// pyListRepr is repr() of a list of str.
func pyListRepr(items []string) string {
	q := make([]string, len(items))
	for i, s := range items {
		q[i] = pyStrRepr(s)
	}
	return "[" + strings.Join(q, ", ") + "]"
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
		return fmt.Sprintf("[Errno %d] %s: %s", int(errno), strerror(errno), pyStrRepr(pe.Path))
	}
	return err.Error()
}
