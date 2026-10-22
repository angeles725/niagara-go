package bog

// References and the XML declaration of the fast well-formedness pass (see wellformed.go).

import (
	"strings"
	"unicode/utf8"
)

// reference checks the reference at '&' i and returns the index after its ';', -1 unless it is
// one of the five predefined entities or a character reference to an XML character.
func reference(s string, i int) int {
	semi := strings.IndexByte(s[i:], ';')
	if semi < 0 {
		return -1
	}
	body := s[i+1 : i+semi]
	end := i + semi + 1
	switch body {
	case "lt", "gt", "amp", "apos", "quot":
		return end
	}
	if len(body) < 2 || body[0] != '#' {
		return -1
	}
	base, digits := 10, body[1:]
	if digits[0] == 'x' {
		base, digits = 16, digits[1:]
	}
	if digits == "" {
		return -1
	}
	v := 0
	for k := 0; k < len(digits); k++ {
		d := digitValue(digits[k], base)
		if d < 0 {
			return -1
		}
		if v = v*base + d; v > utf8.MaxRune {
			return -1
		}
	}
	if !(v == 0x9 || v == 0xA || v == 0xD || 0x20 <= v && v <= 0xD7FF || 0xE000 <= v && v <= 0xFFFD || 0x10000 <= v) {
		return -1
	}
	return end
}

func digitValue(b byte, base int) int {
	switch {
	case '0' <= b && b <= '9':
		return int(b - '0')
	case base == 16 && 'a' <= b && b <= 'f':
		return int(b-'a') + 10
	case base == 16 && 'A' <= b && b <= 'F':
		return int(b-'A') + 10
	}
	return -1
}

// xmlDecl checks an XML declaration of the usual shape at i ("<?xml" then version, optional
// encoding and standalone, as expat's doParseXmlDecl accepts them) and returns the index after
// its "?>"; any other shape is left to pyxml.
func xmlDecl(s string, i int) (int, bool) {
	k := i + len("<?xml")
	var ok bool
	if k, ok = pseudoAttr(s, k, "version"); !ok {
		return 0, false
	}
	v := s[strings.LastIndexAny(s[:k-1], "'\"")+1 : k-1]
	if len(v) < 3 || v[:2] != "1." || strings.Trim(v[2:], "0123456789") != "" {
		return 0, false
	}
	if next, ok := pseudoAttr(s, k, "encoding"); ok {
		e := s[strings.LastIndexAny(s[:next-1], "'\"")+1 : next-1]
		if e == "" || !isASCIILetter(e[0]) {
			return 0, false
		}
		k = next
	}
	if next, ok := pseudoAttr(s, k, "standalone"); ok {
		if sv := s[strings.LastIndexAny(s[:next-1], "'\"")+1 : next-1]; sv != "yes" && sv != "no" {
			return 0, false
		}
		k = next
	}
	k = skipSpace(s, k)
	if !strings.HasPrefix(s[k:], "?>") {
		return 0, false
	}
	return k + 2, true
}

// pseudoAttr checks S name S? = S? quoted value at k (value characters [A-Za-z0-9._-]) and
// returns the index after the closing quote.
func pseudoAttr(s string, k int, name string) (int, bool) {
	j := skipSpace(s, k)
	if j == k || !strings.HasPrefix(s[j:], name) {
		return 0, false
	}
	j = skipSpace(s, j+len(name))
	if j >= len(s) || s[j] != '=' {
		return 0, false
	}
	j = skipSpace(s, j+1)
	if j >= len(s) || s[j] != '"' && s[j] != '\'' {
		return 0, false
	}
	q := s[j]
	for j++; j < len(s); j++ {
		b := s[j]
		if b == q {
			return j + 1, true
		}
		if !(isASCIILetter(b) || '0' <= b && b <= '9' || b == '.' || b == '-' || b == '_') {
			return 0, false
		}
	}
	return 0, false
}
