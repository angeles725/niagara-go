package greenmaxmatrix

// Python text semantics the matrix relies on. decodeReplace is a copy of the obixnav helper (a
// tool never imports another tool's package); it belongs in a shared internal package once one
// exists.

import (
	"errors"
	"fmt"
	"html"
	"io/fs"
	"strconv"
	"strings"
	"syscall"
	"unicode"
	"unicode/utf8"
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

// isPySpace is Python str.isspace.
func isPySpace(r rune) bool { return unicode.IsSpace(r) || r >= 0x1c && r <= 0x1f }

func pyStrip(s string) string { return strings.TrimFunc(s, isPySpace) }

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

// decodeReplace is Python bytes.decode("utf-8", "replace").
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

// invalidCharrefs are html._invalid_charrefs: numeric references HTML5 remaps.
var invalidCharrefs = map[int]string{0x00: "�", 0x0d: "\r", 0x80: "€", 0x81: "\u0081", 0x82: "‚",
	0x83: "ƒ", 0x84: "„", 0x85: "…", 0x86: "†", 0x87: "‡", 0x88: "ˆ", 0x89: "‰", 0x8a: "Š", 0x8b: "‹",
	0x8c: "Œ", 0x8d: "\u008d", 0x8e: "Ž", 0x8f: "\u008f", 0x90: "\u0090", 0x91: "‘", 0x92: "’", 0x93: "“",
	0x94: "”", 0x95: "•", 0x96: "–", 0x97: "—", 0x98: "˜", 0x99: "™", 0x9a: "š", 0x9b: "›", 0x9c: "œ",
	0x9d: "\u009d", 0x9e: "ž", 0x9f: "Ÿ"}

// invalidCodepoint is html._invalid_codepoints: references that decode to nothing.
func invalidCodepoint(n int) bool {
	return n >= 0x1 && n <= 0x8 || n == 0xb || n >= 0xe && n <= 0x1f || n >= 0x7f && n <= 0x9f ||
		n >= 0xfdd0 && n <= 0xfdef || n&0xfffe == 0xfffe
}

// pyUnescape is Python html.unescape: numeric references with the HTML5 remapping, named
// references (with or without ';', longest known prefix) through the HTML5 entity table.
func pyUnescape(s string) string {
	if !strings.Contains(s, "&") {
		return s
	}
	var b strings.Builder
	for {
		i := strings.IndexByte(s, '&')
		if i < 0 {
			b.WriteString(s)
			return b.String()
		}
		b.WriteString(s[:i])
		s = s[i:]
		n, repl := charref(s)
		if n == 0 {
			b.WriteByte('&')
			s = s[1:]
			continue
		}
		b.WriteString(repl)
		s = s[n:]
	}
}

// charref matches _charref at the start of s ("&..."): the bytes consumed and the replacement,
// or 0 when no reference starts there.
func charref(s string) (int, string) {
	body := s[1:]
	if strings.HasPrefix(body, "#") {
		digits, base := body[1:], 10
		if strings.HasPrefix(digits, "x") || strings.HasPrefix(digits, "X") {
			digits, base = digits[1:], 16
		}
		end := 0
		for end < len(digits) && isDigit(digits[end], base) {
			end++
		}
		if end == 0 {
			return 0, ""
		}
		consumed := 1 + len(body) - len(digits) + end
		if end < len(digits) && digits[end] == ';' {
			consumed++
		}
		num, err := strconv.ParseInt(strings.TrimLeft(digits[:end], "0"), base, 64)
		if err != nil || num > 0x10ffff {
			return consumed, "�"
		}
		n := int(num)
		if r, ok := invalidCharrefs[n]; ok {
			return consumed, r
		}
		if n >= 0xd800 && n <= 0xdfff {
			return consumed, "�"
		}
		if invalidCodepoint(n) {
			return consumed, ""
		}
		return consumed, string(rune(n))
	}
	end, count := 0, 0
	for _, r := range body {
		if count == 32 || strings.ContainsRune("\t\n\f <&#;", r) {
			break
		}
		end += utf8.RuneLen(r)
		count++
	}
	if end == 0 {
		return 0, ""
	}
	name := body[:end]
	if end < len(body) && body[end] == ';' {
		name += ";"
	}
	return 1 + len(name), html.UnescapeString("&" + name)
}

func isDigit(c byte, base int) bool {
	if c >= '0' && c <= '9' {
		return true
	}
	return base == 16 && (c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F')
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
	return fmt.Sprintf("[Errno %d] %s: %s", int(errno), msg, pyStrRepr(pe.Path))
}

// pyStrRepr is Python repr(str).
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
		case unicode.IsPrint(r):
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
