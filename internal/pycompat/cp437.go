package pycompat

import "strings"

// CP437Upper is the upper half (bytes 0x80-0xFF) of code page 437, the codec zipfile decodes
// an entry name with when its UTF-8 flag is clear.
var CP437Upper = []rune("ÇüéâäàåçêëèïîìÄÅÉæÆôöòûùÿÖÜ¢£¥₧ƒáíóúñÑªº¿⌐¬½¼¡«»░▒▓│┤╡╢╖╕╣║╗╝╜╛┐" +
	"└┴┬├─┼╞╟╚╔╩╦╠═╬╧╨╤╥╙╘╒╓╫╪┘┌█▄▌▐▀αßΓπΣσµτΦΘΩδ∞φε∩≡±≥≤⌠⌡÷≈°∙·√ⁿ²■\u00a0")

// DecodeCP437 is bytes.decode("cp437").
func DecodeCP437(s string) string { return DecodeCodepage(s, CP437Upper) }

// DecodeCodepage is bytes.decode of a single-byte code page whose lower half is ASCII: bytes
// below 0x80 as is, the others by upper (128 runes). Every byte decodes; ASCII returns s.
func DecodeCodepage(s string, upper []rune) string {
	i := 0
	for i < len(s) && s[i] < 0x80 {
		i++
	}
	if i == len(s) {
		return s
	}
	var b strings.Builder
	b.WriteString(s[:i])
	for ; i < len(s); i++ {
		if c := s[i]; c < 0x80 {
			b.WriteByte(c)
		} else {
			b.WriteRune(upper[c-0x80])
		}
	}
	return b.String()
}
