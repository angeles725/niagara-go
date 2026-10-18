package helpbuild

import (
	"html"
	"strings"
	"unicode/utf8"

	"github.com/angeles725/niagara-go/internal/pycompat"
)

// Character references for the html.parser port (pyhtml.go): html.unescape
// for data and the attribute-value rule of _unescape_attrvalue.

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
			b.WriteString(htmlUnescape(ref))
		case !strings.HasSuffix(ref, "=") && isHTML5Entity(ref[1:]):
			b.WriteString(htmlUnescape(ref))
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

// isHTML5Entity is `name in html.entities.html5` for an ASCII name of
// letters and digits, optionally ending in ";". Go's html package holds the
// same WHATWG table but does not export it; UnescapeString decodes an exact
// name to its 1 or 2 code points (a legacy name without ";" to 1) and a mere
// prefix match to the value plus the rest of the name (at least 2 more
// bytes with ";", 1 without), so the length of the result tells them apart.
func isHTML5Entity(name string) bool {
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
// (found by TestParityHTML5Entities over all 2,231 names).
var goMissingEntities = map[string]string{"&nGt;": "\u226b\u20d2", "&nLt;": "\u226a\u20d2"}

// htmlUnescape is html.unescape: pycompat.HTMLUnescape plus goMissingEntities.
// html.unescape's reference token never holds a second "&", so each "&nGt;"
// is one whole token and the text between them unescapes on its own.
func htmlUnescape(s string) string {
	if !strings.Contains(s, "&nGt;") && !strings.Contains(s, "&nLt;") {
		return pycompat.HTMLUnescape(s)
	}
	var b strings.Builder
	start := 0
	for i := 0; i+5 <= len(s); i++ {
		if v, ok := goMissingEntities[s[i:i+5]]; ok && s[i] == '&' {
			b.WriteString(pycompat.HTMLUnescape(s[start:i]))
			b.WriteString(v)
			start, i = i+5, i+4
		}
	}
	b.WriteString(pycompat.HTMLUnescape(s[start:]))
	return b.String()
}
