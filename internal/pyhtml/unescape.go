package pyhtml

import (
	"html"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"
)

// Character references: html.unescape (Unescape), `name in
// html.entities.html5` (IsHTML5Entity) and the attribute-value rule of
// html.parser._unescape_attrvalue.

var charrefRE = regexp.MustCompile(`&(#[0-9]+;?|#[xX][0-9a-fA-F]+;?|[^\t\n\f <&#;]{1,32};?)`)

// cp1252 is html._invalid_charrefs for the numeric references 0x80..0x9f.
var cp1252 = [32]rune{0x20ac, 0x81, 0x201a, 0x192, 0x201e, 0x2026, 0x2020, 0x2021, 0x2c6, 0x2030, 0x160,
	0x2039, 0x152, 0x8d, 0x17d, 0x8f, 0x90, 0x2018, 0x2019, 0x201c, 0x201d, 0x2022, 0x2013, 0x2014, 0x2dc,
	0x2122, 0x161, 0x203a, 0x153, 0x9d, 0x17e, 0x178}

// invalidCodepoint is html._invalid_codepoints: numeric references to these are dropped.
func invalidCodepoint(n int64) bool {
	return n >= 0x1 && n <= 0x8 || n == 0xb || n >= 0xe && n <= 0x1f || n >= 0x7f && n <= 0x9f ||
		n >= 0xfdd0 && n <= 0xfdef || n&0xfffe == 0xfffe && n <= 0x10ffff
}

// Unescape is html.unescape. A named reference goes through Go's HTML5 table, which
// resolves the longest entity prefix of the token Python's regex matched, like Python,
// plus goMissingEntities; numeric references follow Python's replacement rules. The
// reference token never holds a second "&", so "&nGt;" is always one whole token.
func Unescape(s string) string {
	if !strings.Contains(s, "&") {
		return s
	}
	return charrefRE.ReplaceAllStringFunc(s, func(m string) string {
		ref := m[1:]
		if ref[0] != '#' {
			if v, ok := goMissingEntities[m]; ok {
				return v
			}
			return html.UnescapeString(m)
		}
		digits, base := strings.TrimSuffix(ref[1:], ";"), 10
		if digits[0] == 'x' || digits[0] == 'X' {
			digits, base = digits[1:], 16
		}
		n, err := strconv.ParseInt(digits, base, 64)
		switch {
		case err != nil || n > 0x10ffff || n >= 0xd800 && n <= 0xdfff || n == 0:
			return "\uFFFD"
		case n == 0x0d:
			return "\r"
		case n >= 0x80 && n <= 0x9f:
			return string(cp1252[n-0x80])
		case invalidCodepoint(n):
			return ""
		}
		return string(rune(n))
	})
}

// unescapeAttrValue is html.parser._unescape_attrvalue: the attr_charref
// matches &(#[0-9]+|#[xX][0-9a-fA-F]+|[a-zA-Z][a-zA-Z0-9]*)[;=]?; a numeric
// one is always unescaped, a named one only when it is exactly an HTML5
// entity name and not followed by "=".
func unescapeAttrValue(s string) string {
	if !strings.Contains(s, "&") {
		return s
	}
	var b strings.Builder
	n := len(s)
	for i := 0; i < n; {
		j := attrCharrefEnd(s, i)
		if j < 0 {
			b.WriteByte(s[i])
			i++
			continue
		}
		ref := s[i:j]
		switch {
		case ref[1] == '#':
			b.WriteString(Unescape(ref))
		case !strings.HasSuffix(ref, "=") && IsHTML5Entity(ref[1:]):
			b.WriteString(Unescape(ref))
		default:
			b.WriteString(ref)
		}
		i = j
	}
	return b.String()
}

func isDigitByte(c byte) bool { return c >= '0' && c <= '9' }
func isHexByte(c byte) bool   { return isDigitByte(c) || c|0x20 >= 'a' && c|0x20 <= 'f' }

// attrCharrefEnd is the end of an attr_charref match at i, or -1.
func attrCharrefEnd(s string, i int) int {
	n := len(s)
	if s[i] != '&' || i+1 >= n {
		return -1
	}
	j := i + 1
	switch {
	case s[j] == '#' && j+1 < n && isDigitByte(s[j+1]):
		j += 2
		for j < n && isDigitByte(s[j]) {
			j++
		}
	case s[j] == '#' && j+2 < n && (s[j+1] == 'x' || s[j+1] == 'X') && isHexByte(s[j+2]):
		j += 3
		for j < n && isHexByte(s[j]) {
			j++
		}
	case isASCIILetter(s[j]):
		j++
		for j < n && (isASCIILetter(s[j]) || isDigitByte(s[j])) {
			j++
		}
	default:
		return -1
	}
	if j < n && (s[j] == ';' || s[j] == '=') {
		j++
	}
	return j
}

// IsHTML5Entity is `name in html.entities.html5` for an ASCII name of
// letters and digits, optionally ending in ";". Go's html package holds the
// same WHATWG table but does not export it; UnescapeString decodes an exact
// name to its 1 or 2 code points (a legacy name without ";" to 1) and a mere
// prefix match to the value plus the rest of the name (at least 2 more
// bytes with ";", 1 without), so the length of the result tells them apart.
func IsHTML5Entity(name string) bool {
	if _, ok := goMissingEntities["&"+name]; ok {
		return true
	}
	ref := "&" + name
	out := html.UnescapeString(ref)
	limit := 1
	if strings.HasSuffix(name, ";") {
		limit = 2
	}
	return out != ref && utf8.RuneCountInString(out) <= limit
}

// goMissingEntities are the html.entities.html5 names Go's html table lacks
// (found by helpbuild's TestParityHTML5Entities over all 2,231 names).
var goMissingEntities = map[string]string{"&nGt;": "\u226b\u20d2", "&nLt;": "\u226a\u20d2"}
