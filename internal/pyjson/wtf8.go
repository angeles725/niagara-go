package pyjson

import "unicode/utf8"

// Strings coming out of the JSON decoder are WTF-8: valid UTF-8 plus lone
// surrogates encoded as 3-byte sequences (Python str can hold them), so
// equality and ordering stay byte-wise while every code point round-trips.

// AppendRune appends r as WTF-8 (surrogates included).
func AppendRune(b []byte, r rune) []byte {
	switch {
	case r < 0x80:
		return append(b, byte(r))
	case r < 0x800:
		return append(b, 0xC0|byte(r>>6), 0x80|byte(r&0x3F))
	case r < 0x10000:
		return append(b, 0xE0|byte(r>>12), 0x80|byte(r>>6&0x3F), 0x80|byte(r&0x3F))
	default:
		return append(b, 0xF0|byte(r>>18), 0x80|byte(r>>12&0x3F), 0x80|byte(r>>6&0x3F), 0x80|byte(r&0x3F))
	}
}

// Runes decodes a WTF-8 string into code points: every valid UTF-8 sequence and every
// 3-byte lone surrogate (ED A0..BF 80..BF) is one code point; any other byte is one U+FFFD and
// decoding resumes at the next byte, so a bad byte never swallows the text after it.
func Runes(s string) []rune {
	out := make([]rune, 0, len(s))
	for i := 0; i < len(s); {
		if c := s[i]; c < 0x80 {
			out = append(out, rune(c))
			i++
			continue
		}
		if r, size := utf8.DecodeRuneInString(s[i:]); r != utf8.RuneError || size > 1 {
			out = append(out, r)
			i += size
			continue
		}
		if i+2 < len(s) && s[i] == 0xED && s[i+1]&0xE0 == 0xA0 && s[i+2]&0xC0 == 0x80 {
			out = append(out, 0xD000|rune(s[i+1]&0x3F)<<6|rune(s[i+2]&0x3F))
			i += 3
			continue
		}
		out = append(out, utf8.RuneError)
		i++
	}
	return out
}

// WTF8 encodes code points (lone surrogates included) as a WTF-8 string.
func WTF8(rs []rune) string {
	b := make([]byte, 0, len(rs))
	for _, r := range rs {
		b = AppendRune(b, r)
	}
	return string(b)
}
