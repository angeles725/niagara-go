package bog

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Python text semantics the parser depends on, reproduced exactly (Python 3.15 and Go 1.27 both
// use Unicode 17 tables).

// decodeReplace is bytes.decode("utf-8", errors="replace"): one U+FFFD per maximal subpart of an
// ill-formed sequence (Go's utf8 decoder yields one per byte instead).
func decodeReplace(b []byte) string {
	if utf8.Valid(b) {
		return string(b)
	}
	var sb strings.Builder
	sb.Grow(len(b) + 16)
	for i := 0; i < len(b); {
		c := b[i]
		if c < utf8.RuneSelf {
			sb.WriteByte(c)
			i++
			continue
		}
		need, lo, hi := seqShape(c)
		if need == 0 {
			sb.WriteRune(utf8.RuneError)
			i++
			continue
		}
		j := i + 1
		for k := 0; k < need; k++ {
			if j >= len(b) {
				break
			}
			low, high := byte(0x80), byte(0xBF)
			if k == 0 {
				low, high = lo, hi
			}
			if b[j] < low || b[j] > high {
				break
			}
			j++
		}
		if j-i == need+1 {
			sb.Write(b[i:j])
		} else {
			sb.WriteRune(utf8.RuneError)
		}
		i = j
	}
	return sb.String()
}

// seqShape returns how many continuation bytes lead byte c needs and the valid range of the first
// one; need is 0 for a byte that cannot start a sequence.
func seqShape(c byte) (need int, lo, hi byte) {
	switch {
	case c >= 0xC2 && c <= 0xDF:
		return 1, 0x80, 0xBF
	case c == 0xE0:
		return 2, 0xA0, 0xBF
	case c == 0xED:
		return 2, 0x80, 0x9F
	case c >= 0xE1 && c <= 0xEF:
		return 2, 0x80, 0xBF
	case c == 0xF0:
		return 3, 0x90, 0xBF
	case c >= 0xF1 && c <= 0xF3:
		return 3, 0x80, 0xBF
	case c == 0xF4:
		return 3, 0x80, 0x8F
	}
	return 0, 0, 0
}

// pyRepr is Python's repr() of a str.
func pyRepr(s string) string {
	quote := byte('\'')
	if strings.IndexByte(s, '\'') >= 0 && strings.IndexByte(s, '"') < 0 {
		quote = '"'
	}
	var sb strings.Builder
	sb.WriteByte(quote)
	for _, r := range s {
		switch {
		case r == '\\':
			sb.WriteString(`\\`)
		case r == rune(quote):
			sb.WriteByte('\\')
			sb.WriteByte(quote)
		case r == '\t':
			sb.WriteString(`\t`)
		case r == '\n':
			sb.WriteString(`\n`)
		case r == '\r':
			sb.WriteString(`\r`)
		case unicode.IsPrint(r): // str.isprintable: L, M, N, P, S and ' '
			sb.WriteRune(r)
		case r < 0x100:
			fmt.Fprintf(&sb, `\x%02x`, r)
		case r < 0x10000:
			fmt.Fprintf(&sb, `\u%04x`, r)
		default:
			fmt.Fprintf(&sb, `\U%08x`, r)
		}
	}
	sb.WriteByte(quote)
	return sb.String()
}

// isWordRune is re's \w on str patterns: str.isalnum() or '_' (Unicode letters and numbers).
func isWordRune(r rune) bool {
	if r < utf8.RuneSelf {
		return r == '_' || 'a' <= r && r <= 'z' || 'A' <= r && r <= 'Z' || '0' <= r && r <= '9'
	}
	return unicode.IsLetter(r) || unicode.IsNumber(r)
}

// isPySpace is str.isspace(): Go's unicode.IsSpace plus the ASCII separators U+001C..U+001F.
func isPySpace(r rune) bool {
	return unicode.IsSpace(r) || r >= 0x1C && r <= 0x1F
}

// isLineBreak reports a str.splitlines() boundary.
func isLineBreak(r rune) bool {
	switch r {
	case '\n', '\r', '\v', '\f', 0x1C, 0x1D, 0x1E, 0x85, 0x2028, 0x2029:
		return true
	}
	return false
}

// pyFields is str.split() with no separator.
func pyFields(s string) []string {
	return strings.FieldsFunc(s, isPySpace)
}

// lineBreakAt returns the byte length of the splitlines() boundary at s[i:], or 0.
func lineBreakAt(s string, i int) int {
	switch c := s[i]; c {
	case '\n', '\r', '\v', '\f', 0x1C, 0x1D, 0x1E:
		return 1
	case 0xC2:
		if i+1 < len(s) && s[i+1] == 0x85 {
			return 2
		}
	case 0xE2:
		if i+2 < len(s) && s[i+1] == 0x80 && (s[i+2] == 0xA8 || s[i+2] == 0xA9) {
			return 3
		}
	}
	return 0
}
