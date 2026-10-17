package pycompat

import (
	"strings"
	"unicode/utf8"

	"github.com/angeles725/niagara-go/internal/textfold"
)

const (
	capitalSigma = 'Σ'
	smallSigma   = 'σ'
	finalSigma   = 'ς'
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

// IsLower is str.islower(): at least one cased character and no upper-case or title-case one.
func IsLower(s string) bool {
	t := data()
	if r, ok := single(s); ok {
		return in(t.lower, r)
	}
	cased := false
	for _, r := range s {
		if in(t.upper, r) || in(t.title, r) {
			return false
		}
		cased = cased || in(t.lower, r)
	}
	return cased
}

// IsTitle is str.istitle(): upper-case and title-case characters only follow uncased ones,
// lower-case characters only follow cased ones, and there is at least one cased character.
func IsTitle(s string) bool {
	t := data()
	if r, ok := single(s); ok {
		return in(t.title, r) || in(t.upper, r)
	}
	cased, prevCased := false, false
	for _, r := range s {
		switch {
		case in(t.upper, r) || in(t.title, r):
			if prevCased {
				return false
			}
			prevCased, cased = true, true
		case in(t.lower, r):
			if !prevCased {
				return false
			}
			prevCased, cased = true, true
		default:
			prevCased = false
		}
	}
	return cased
}

// Upper is str.upper(): the full mapping, so "ß" becomes "SS".
func Upper(s string) string {
	t := data()
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		writeMapped(&b, t.upperMap, r)
	}
	return b.String()
}

// Lower is str.lower(), final sigma included (textfold.Lower).
func Lower(s string) string { return textfold.Lower(s) }

// Title is str.title(): a character after a cased one is lowered, any other is title-cased.
func Title(s string) string {
	t := data()
	runes := []rune(s)
	var b strings.Builder
	b.Grow(len(s))
	prevCased := false
	for i, r := range runes {
		if prevCased {
			lowerAt(&b, t, runes, i)
		} else {
			writeMapped(&b, t.titleMap, r)
		}
		prevCased = in(t.cased, r)
	}
	return b.String()
}

// Capitalize is str.capitalize(): the first character title-cased, the rest lowered.
func Capitalize(s string) string {
	t := data()
	runes := []rune(s)
	var b strings.Builder
	b.Grow(len(s))
	for i, r := range runes {
		if i == 0 {
			writeMapped(&b, t.titleMap, r)
		} else {
			lowerAt(&b, t, runes, i)
		}
	}
	return b.String()
}

// SwapCase is str.swapcase(): upper-case characters lowered, lower-case ones upper-cased.
func SwapCase(s string) string {
	t := data()
	runes := []rune(s)
	var b strings.Builder
	b.Grow(len(s))
	for i, r := range runes {
		switch {
		case in(t.upper, r):
			lowerAt(&b, t, runes, i)
		case in(t.lower, r):
			writeMapped(&b, t.upperMap, r)
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

func writeMapped(b *strings.Builder, m map[rune]string, r rune) {
	if to, ok := m[r]; ok {
		b.WriteString(to)
	} else {
		b.WriteRune(r)
	}
}

// lowerAt writes the str.lower() of runes[i] in its context (CPython lower_ucs4).
func lowerAt(b *strings.Builder, t *tables, runes []rune, i int) {
	switch r := runes[i]; {
	case r < utf8.RuneSelf:
		if 'A' <= r && r <= 'Z' {
			r += 'a' - 'A'
		}
		b.WriteByte(byte(r))
	case r == capitalSigma:
		if isFinalSigma(t, runes, i) {
			b.WriteRune(finalSigma)
		} else {
			b.WriteRune(smallSigma)
		}
	default:
		b.WriteString(textfold.Lower(string(r)))
	}
}

// isFinalSigma: \p{cased}\p{case-ignorable}* U+03A3 !(\p{case-ignorable}* \p{cased}).
func isFinalSigma(t *tables, runes []rune, i int) bool {
	j := i - 1
	for j >= 0 && in(t.ignorable, runes[j]) {
		j--
	}
	if j < 0 || !in(t.cased, runes[j]) {
		return false
	}
	j = i + 1
	for j < len(runes) && in(t.ignorable, runes[j]) {
		j++
	}
	return j == len(runes) || !in(t.cased, runes[j])
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

// Center is str.center(width, fill). An odd margin puts the extra fill on the left when
// width is odd and on the right when it is even (unlike format's "^", always right).
func Center(s string, width int, fill rune) string {
	marg := width - utf8.RuneCountInString(s)
	if marg <= 0 {
		return s
	}
	left := marg/2 + (marg & width & 1)
	f := string(fill)
	return strings.Repeat(f, left) + s + strings.Repeat(f, marg-left)
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
