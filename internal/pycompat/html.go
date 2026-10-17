package pycompat

import (
	"html"
	"regexp"
	"strconv"
	"strings"
)

var htmlEscaper = strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;", "'", "&#x27;")

// HTMLEscape is html.escape(s, quote=True).
func HTMLEscape(s string) string { return htmlEscaper.Replace(s) }

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

// HTMLUnescape is html.unescape. Named references go through Go's HTML5 table, which
// resolves the longest entity prefix of the token Python's regex matched, like Python;
// numeric references follow Python's replacement rules.
func HTMLUnescape(s string) string {
	if !strings.Contains(s, "&") {
		return s
	}
	return charrefRE.ReplaceAllStringFunc(s, func(m string) string {
		ref := m[1:]
		if ref[0] != '#' {
			return html.UnescapeString(m)
		}
		digits, base := strings.TrimSuffix(ref[1:], ";"), 10
		if digits[0] == 'x' || digits[0] == 'X' {
			digits, base = digits[1:], 16
		}
		n, err := strconv.ParseInt(digits, base, 64)
		switch {
		case err != nil || n > 0x10ffff || n >= 0xd800 && n <= 0xdfff || n == 0:
			return "�"
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
