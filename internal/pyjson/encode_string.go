package pyjson

import (
	"strings"
	"unicode/utf8"
)

// plain[ensureASCII][c] reports that ASCII byte c is copied to the output as is:
// json escapes '"', '\\' and the C0 controls always, and DEL (0x7f) too when
// ensure_ascii (its ESCAPE_ASCII class is [^\ -~]). Bytes >= 0x80 are never plain:
// they start a code point that is validated (and, when ensure_ascii, escaped).
var plain = func() (t [2][256]bool) {
	for c := 0x20; c < 0x80; c++ {
		t[0][c] = c != '"' && c != '\\'
		t[1][c] = t[0][c] && c != 0x7f
	}
	return t
}()

const hexDigits = "0123456789abcdef"

// EncodeString mirrors json's py_encode_basestring(_ascii).
func EncodeString(s string, ensureASCII bool) string {
	if plainRun(s, 0, ensureASCII) == len(s) {
		return `"` + s + `"`
	}
	var b strings.Builder
	b.Grow(len(s) + 16)
	writeString(&b, s, ensureASCII)
	return b.String()
}

// plainRun returns the end of the run of plain ASCII bytes of s starting at i.
func plainRun(s string, i int, ensureASCII bool) int {
	t := &plain[0]
	if ensureASCII {
		t = &plain[1]
	}
	for i < len(s) && t[s[i]] {
		i++
	}
	return i
}

// writeString writes s quoted as json.dumps does. s is WTF-8 and is read code
// point by code point as Runes reads it: valid UTF-8 and 3-byte lone surrogates
// are one code point each, any other byte is one U+FFFD. Runs that need no
// escaping are copied straight from s.
func writeString(b *strings.Builder, s string, ensureASCII bool) {
	b.WriteByte('"')
	for i := 0; i < len(s); {
		j := plainRun(s, i, ensureASCII)
		if !ensureASCII {
			// Valid UTF-8 and lone surrogates re-encode to their own bytes, so
			// they extend the run; only an invalid byte (U+FFFD) breaks it.
			for j < len(s) && s[j] >= 0x80 {
				r, size := decodeWTF8(s, j)
				if r == utf8.RuneError && size == 1 {
					break
				}
				j = plainRun(s, j+size, false)
			}
		}
		b.WriteString(s[i:j])
		if j == len(s) {
			break
		}
		r, size := decodeWTF8(s, j)
		writeEscaped(b, r, ensureASCII)
		i = j + size
	}
	b.WriteByte('"')
}

// decodeWTF8 decodes the code point at s[i:] as Runes does.
func decodeWTF8(s string, i int) (rune, int) {
	if c := s[i]; c < 0x80 {
		return rune(c), 1
	}
	if r, size := utf8.DecodeRuneInString(s[i:]); r != utf8.RuneError || size > 1 {
		return r, size
	}
	if i+2 < len(s) && s[i] == 0xED && s[i+1]&0xE0 == 0xA0 && s[i+2]&0xC0 == 0x80 {
		return 0xD000 | rune(s[i+1]&0x3F)<<6 | rune(s[i+2]&0x3F), 3
	}
	return utf8.RuneError, 1
}

// writeEscaped writes one code point that is not plain: a json escape, or (only
// when !ensureASCII, for the U+FFFD of an invalid byte) its UTF-8 bytes.
func writeEscaped(b *strings.Builder, r rune, ensureASCII bool) {
	switch r {
	case '"':
		b.WriteString(`\"`)
	case '\\':
		b.WriteString(`\\`)
	case '\n':
		b.WriteString(`\n`)
	case '\r':
		b.WriteString(`\r`)
	case '\t':
		b.WriteString(`\t`)
	case '\b':
		b.WriteString(`\b`)
	case '\f':
		b.WriteString(`\f`)
	default:
		switch {
		case r < 0x20:
			writeU(b, r)
		case r < 0x7F || !ensureASCII:
			var buf [4]byte
			b.Write(AppendRune(buf[:0], r))
		case r > 0xFFFF:
			r -= 0x10000
			writeU(b, 0xD800|(r>>10)&0x3FF)
			writeU(b, 0xDC00|r&0x3FF)
		default:
			writeU(b, r)
		}
	}
}

// writeU writes the escape \uXXXX of the 16-bit unit u in lowercase hex.
func writeU(b *strings.Builder, u rune) {
	b.Write([]byte{'\\', 'u', hexDigits[u>>12&0xF], hexDigits[u>>8&0xF], hexDigits[u>>4&0xF], hexDigits[u&0xF]})
}
