package pycompat

import (
	"strings"
	"unicode/utf8"
)

func in(rs []runeRange, r rune) bool { return find(rs, r) >= 0 }

// single returns the only code point of s, ok false when s has zero or several.
func single(s string) (rune, bool) {
	r, n := utf8.DecodeRuneInString(s)
	return r, n > 0 && n == len(s)
}

// IsUpper is str.isupper(): at least one cased character and no lower-case or title-case one.
func IsUpper(s string) bool {
	t := data()
	if r, ok := single(s); ok {
		return in(t.upper, r)
	}
	cased := false
	for _, r := range s {
		if in(t.lower, r) || in(t.title, r) {
			return false
		}
		cased = cased || in(t.upper, r)
	}
	return cased
}

// LJust is str.ljust(width, fill): s padded on the right to width code points.
func LJust(s string, width int, fill rune) string {
	n := utf8.RuneCountInString(s)
	if width <= n {
		return s
	}
	return s + strings.Repeat(string(fill), width-n)
}

// RJust is str.rjust(width, fill): s padded on the left to width code points.
func RJust(s string, width int, fill rune) string {
	n := utf8.RuneCountInString(s)
	if width <= n {
		return s
	}
	return strings.Repeat(string(fill), width-n) + s
}

// Cut is s[:n] by code points; a negative n counts from the end like a Python slice.
func Cut(s string, n int) string {
	if n < 0 {
		n += utf8.RuneCountInString(s)
		if n <= 0 {
			return ""
		}
	}
	for i := range s {
		if n == 0 {
			return s[:i]
		}
		n--
	}
	return s
}
