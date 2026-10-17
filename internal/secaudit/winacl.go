package secaudit

// Port of tools/lib/winacl.py: reading a Windows icacls listing (ACE lines, broad groups with
// write rights) and decoding a console tool's output.

import (
	"regexp"
	"strings"
	"unicode"
	"unicode/utf16"
	"unicode/utf8"
)

// aceRE is one ACE line: `[<path> ]<principal>:(<inheritance>)...(<rights>)`. The rights letters
// are the same in every Windows language and read case-insensitively.
var aceRE = regexp.MustCompile(pyRe(`(?i)^(.*?):((?:\([A-Z]+(?:,[A-Z]+)*\))+)\s*$`))

var groupRE = regexp.MustCompile(`\(([A-Za-z,]+)\)`)

// broadGroupRE names the broad groups in English, Spanish, Portuguese, French, German and Italian.
var broadGroupRE = regexp.MustCompile(pyRe(`(?i)(?:^|[\s\\])(Authenticated Users|Users|Everyone` +
	`|Usuarios autentificados|Usuarios autenticados|Usuarios|Todos` +
	`|Usuários autenticados|Usuários` +
	`|Utilisateurs authentifiés|Utilisateurs|Tout le monde` +
	`|Authentifizierte Benutzer|Benutzer|Jeder` +
	`|Utenti autenticati|Utenti)\n?$`))

// writeRights are the rights that let their holder change what security/ holds: F, M, W, D,
// generic GA/GW and the specific WD/AD/WEA/WA/DE/DC/WDAC/WO.
var writeRights = map[string]bool{"F": true, "M": true, "W": true, "D": true, "GA": true, "GW": true,
	"WD": true, "AD": true, "WEA": true, "WA": true, "DE": true, "DC": true, "WDAC": true, "WO": true}

// ace is one parsed ACE: the principal and its upper-cased parenthesised groups.
type ace struct {
	who    string
	rights []string
}

// parseListing returns the ACEs of an icacls listing and the ACE-looking lines (holding ":(")
// it did not understand. The queried path is cut from the line that starts with it.
func parseListing(text, winpath string) (aces []ace, unparsed []string) {
	for _, line := range strings.Split(normalizeNewlines(text), "\n") {
		line = pyStrip(line)
		if winpath != "" && strings.HasPrefix(strings.ToLower(line), strings.ToLower(winpath)) {
			line = pyStrip(string([]rune(line)[utf8.RuneCountInString(winpath):]))
		}
		m := aceRE.FindStringSubmatch(line)
		if m != nil && m[1] != "" {
			var groups []string
			for _, g := range groupRE.FindAllStringSubmatch(m[2], -1) {
				groups = append(groups, strings.ToUpper(g[1]))
			}
			aces = append(aces, ace{pyStrip(m[1]), groups})
		} else if strings.Contains(line, ":(") {
			unparsed = append(unparsed, line)
		}
	}
	return aces, unparsed
}

// normalizeNewlines maps the line boundaries str.splitlines knows to "\n".
func normalizeNewlines(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	return strings.Map(func(r rune) rune {
		switch r {
		case '\r', '\v', '\f', 0x1c, 0x1d, 0x1e, 0x85, 0x2028, 0x2029:
			return '\n'
		}
		return r
	}, s)
}

// looseACEs are the principals naming a broad group that hold a write right.
func looseACEs(aces []ace) []string {
	var out []string
	for _, a := range aces {
		if !broadGroupRE.MatchString(a.who) {
			continue
		}
		for _, g := range a.rights {
			if anyWrite(g) {
				out = append(out, a.who)
				break
			}
		}
	}
	return out
}

func anyWrite(group string) bool {
	for _, r := range strings.Split(group, ",") {
		if writeRights[r] {
			return true
		}
	}
	return false
}

// The upper halves (0x80-0xff) of the console code pages icacls may print in, in tie order:
// cp850 (Spanish console), cp437 (English console), cp1252 (ANSI, a redirected console). Bytes a
// code page does not define decode to U+FFFD.
var codepages = []struct {
	name  string
	upper []rune
}{
	{"cp850", []rune("ÇüéâäàåçêëèïîìÄÅÉæÆôöòûùÿÖÜø£Ø×ƒáíóúñÑªº¿®¬½¼¡«»░▒▓│┤ÁÂÀ©╣║╗╝¢¥┐└┴┬├─┼ãÃ╚╔╩╦╠═╬¤ðÐÊËÈıÍÎÏ┘┌█▄¦Ì▀ÓßÔÒõÕµþÞÚÛÙýÝ¯´­±‗¾¶§÷¸°¨·¹³²■ ")},
	{"cp437", []rune("ÇüéâäàåçêëèïîìÄÅÉæÆôöòûùÿÖÜ¢£¥₧ƒáíóúñÑªº¿⌐¬½¼¡«»░▒▓│┤╡╢╖╕╣║╗╝╜╛┐└┴┬├─┼╞╟╚╔╩╦╠═╬╧╨╤╥╙╘╒╓╫╪┘┌█▄▌▐▀αßΓπΣσµτΦΘΩδ∞φε∩≡±≥≤⌠⌡÷≈°∙·√ⁿ²■ ")},
	{"cp1252", []rune("€\ufffd‚ƒ„…†‡ˆ‰Š‹Œ\ufffdŽ\ufffd\ufffd‘’“”•–—˜™š›œ\ufffdžŸ" +
		" ¡¢£¤¥¦§¨©ª«¬­®¯°±²³´µ¶·¸¹º»¼½¾¿ÀÁÂÃÄÅÆÇÈÉÊËÌÍÎÏÐÑÒÓÔÕÖ×ØÙÚÛÜÝÞßàáâãäåæçèéêëìíîïðñòóôõö÷øùúûüýþÿ")},
}

func decodeCodepage(data []byte, upper []rune) string {
	var b strings.Builder
	for _, c := range data {
		if c < 0x80 {
			b.WriteByte(c)
		} else {
			b.WriteRune(upper[c-0x80])
		}
	}
	return b.String()
}

// frequent are the letters a Western-European Windows listing commonly holds.
var frequent = func() map[rune]bool {
	m := map[rune]bool{}
	for _, r := range "áéíóúñüçàèâêôãõäöÁÉÍÓÚÑÜÇÀÈÂÊÔÃÕÄÖ" {
		m[r] = true
	}
	return m
}()

func isASCII(r rune) bool { return r < 0x80 }

// plausibility scores a decoding over its non-ASCII characters: a frequent letter +3, another
// letter +1, a letter in the wrong case for its neighbours -1, anything else -1.
func plausibility(text string) int {
	rs := []rune(text)
	score := 0
	for i, ch := range rs {
		if ch < 0x80 {
			continue
		}
		prev, next := rune(-1), rune(-1)
		if i > 0 {
			prev = rs[i-1]
		}
		if i+1 < len(rs) {
			next = rs[i+1]
		}
		upperAfterLower := unicode.IsUpper(ch) && prev >= 0 && isASCII(prev) && unicode.IsLower(prev)
		lowerBetweenUppers := unicode.IsLower(ch) && prev >= 0 && isASCII(prev) && unicode.IsUpper(prev) &&
			next >= 0 && isASCII(next) && unicode.IsUpper(next)
		switch {
		case !unicode.IsLetter(ch):
			score--
		case upperAfterLower || lowerBetweenUppers:
			score--
		case frequent[ch]:
			score += 3
		default:
			score++
		}
	}
	return score
}

// decodeConsole is the text of a Windows console tool's output: UTF-16 with a BOM, UTF-8, else
// the code page decoding that reads as the most plausible text (the first code page on a tie).
func decodeConsole(data []byte) string {
	if len(data) >= 2 && (data[0] == 0xff && data[1] == 0xfe || data[0] == 0xfe && data[1] == 0xff) {
		return decodeUTF16(data)
	}
	if utf8.Valid(data) {
		return string(data)
	}
	best, bestScore := "", 0
	for i, cp := range codepages {
		text := decodeCodepage(data, cp.upper)
		if s := plausibility(text); i == 0 || s > bestScore {
			best, bestScore = text, s
		}
	}
	return best
}

// decodeUTF16 is bytes.decode("utf-16", "replace") of data starting with a BOM.
func decodeUTF16(data []byte) string {
	le := data[0] == 0xff
	data = data[2:]
	units := make([]uint16, 0, len(data)/2)
	for i := 0; i+1 < len(data); i += 2 {
		if le {
			units = append(units, uint16(data[i])|uint16(data[i+1])<<8)
		} else {
			units = append(units, uint16(data[i])<<8|uint16(data[i+1]))
		}
	}
	text := string(utf16.Decode(units))
	if len(data)%2 == 1 {
		text += "\ufffd"
	}
	return text
}
