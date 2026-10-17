package argparse

import (
	"strings"
	"unicode/utf8"

	"github.com/angeles725/niagara-go/internal/pycompat"
)

// Text helpers of HelpFormatter; the Python value and str semantics live in pycompat.

// isASCIISpace is the re.ASCII \s class.
func isASCIISpace(r rune) bool {
	return r == ' ' || r == '\t' || r == '\n' || r == '\r' || r == '\f' || r == '\v'
}

// normalizeSpace is re.sub(r'\s+', ' ', text, flags=re.ASCII).strip().
func normalizeSpace(text string) string {
	var b strings.Builder
	inSpace := false
	for _, r := range text {
		if isASCIISpace(r) {
			if !inSpace {
				b.WriteByte(' ')
			}
			inSpace = true
			continue
		}
		inSpace = false
		b.WriteRune(r)
	}
	return pycompat.Strip(b.String())
}

// runeLen is Python len(str).
func runeLen(s string) int { return utf8.RuneCountInString(s) }

// padRight is '%-*s' % (width, s).
func padRight(s string, width int) string {
	if n := runeLen(s); n < width {
		return s + strings.Repeat(" ", width-n)
	}
	return s
}
