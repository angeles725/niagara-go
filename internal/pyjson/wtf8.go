package pyjson

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

// Runes decodes a WTF-8 string produced by this package into code points.
func Runes(s string) []rune {
	out := make([]rune, 0, len(s))
	for i := 0; i < len(s); {
		c := s[i]
		switch {
		case c < 0x80:
			out = append(out, rune(c))
			i++
		case c < 0xE0 && i+1 < len(s):
			out = append(out, rune(c&0x1F)<<6|rune(s[i+1]&0x3F))
			i += 2
		case c < 0xF0 && i+2 < len(s):
			out = append(out, rune(c&0x0F)<<12|rune(s[i+1]&0x3F)<<6|rune(s[i+2]&0x3F))
			i += 3
		case i+3 < len(s):
			out = append(out, rune(c&0x07)<<18|rune(s[i+1]&0x3F)<<12|rune(s[i+2]&0x3F)<<6|rune(s[i+3]&0x3F))
			i += 4
		default:
			out = append(out, 0xFFFD)
			i++
		}
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
