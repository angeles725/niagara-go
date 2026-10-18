package core

import (
	"archive/zip"
	"strings"
)

// ZipName is the name zipfile.ZipFile.namelist() gives an entry: UTF-8 when
// the entry's flag says so, else decoded as cp437, cut at the first NUL.
func ZipName(f *zip.File) string {
	n := f.Name
	if f.Flags&0x800 == 0 {
		n = decodeCP437(n)
	}
	if k := strings.IndexByte(n, 0); k >= 0 {
		n = n[:k]
	}
	return n
}

// cp437High maps bytes 0x80-0xFF of code page 437 to their characters.
const cp437High = "ÇüéâäàåçêëèïîìÄÅÉæÆôöòûùÿÖÜ¢£¥₧ƒáíóúñÑªº¿⌐¬½¼¡«»░▒▓│┤╡╢╖╕╣║╗╝╜╛┐└┴┬├─┼╞╟╚╔╩╦╠═╬╧╨╤╥╙╘╒╓╫╪┘┌█▄▌▐▀αßΓπΣσµτΦΘΩδ∞φε∩≡±≥≤⌠⌡÷≈°∙·√ⁿ²■ "

var cp437Runes = []rune(cp437High)

// decodeCP437 is bytes.decode("cp437"): ASCII as is, the high half by table.
func decodeCP437(s string) string {
	ascii := true
	for i := 0; i < len(s); i++ {
		if s[i] >= 0x80 {
			ascii = false
			break
		}
	}
	if ascii {
		return s
	}
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if c := s[i]; c < 0x80 {
			b.WriteByte(c)
		} else {
			b.WriteRune(cp437Runes[c-0x80])
		}
	}
	return b.String()
}
