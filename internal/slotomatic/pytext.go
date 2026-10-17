package slotomatic

import (
	"os"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/angeles725/niagara-go/internal/textfold"
)

// IsSpace is Python's str.isspace() for one rune (re's \s on a str pattern).
func IsSpace(r rune) bool { return textfold.IsSpace(r) }

// IsWord is re's \w on a str pattern: str.isalnum() or '_'.
func IsWord(r rune) bool { return r == '_' || unicode.IsLetter(r) || unicode.IsNumber(r) }

// Strip is Python's str.strip() with no argument.
func Strip(s string) string { return strings.TrimFunc(s, IsSpace) }

func decodeRune(s string) (rune, int) { return utf8.DecodeRuneInString(s) }

// ReadSource reads a source file the way open(path, encoding="utf-8", errors="replace").read()
// does: undecodable bytes become U+FFFD (one per maximal invalid subpart, like CPython) and
// "\r\n" and "\r" become "\n" (universal newlines).
func ReadSource(path string) (string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return UniversalNewlines(DecodeReplace(raw)), nil
}

// UniversalNewlines turns "\r\n" and a lone "\r" into "\n", as Python's text mode does.
func UniversalNewlines(s string) string {
	if !strings.ContainsRune(s, '\r') {
		return s
	}
	return strings.ReplaceAll(strings.ReplaceAll(s, "\r\n", "\n"), "\r", "\n")
}

// DecodeReplace is bytes.decode("utf-8", errors="replace").
func DecodeReplace(b []byte) string {
	if utf8.Valid(b) {
		return string(b)
	}
	var sb strings.Builder
	sb.Grow(len(b) + 16)
	for i := 0; i < len(b); {
		if r, n := utf8.DecodeRune(b[i:]); r != utf8.RuneError || n > 1 {
			sb.WriteRune(r)
			i += n
			continue
		}
		sb.WriteRune(utf8.RuneError)
		i += invalidPrefix(b[i:])
	}
	return sb.String()
}

// invalidPrefix is the length of the maximal subpart of an ill-formed sequence at b[0]: the
// lead byte plus the continuation bytes that are still valid for it (at least 1).
func invalidPrefix(b []byte) int {
	c := b[0]
	var need int
	lo, hi := byte(0x80), byte(0xBF)
	switch {
	case c >= 0xC2 && c <= 0xDF:
		need = 1
	case c == 0xE0:
		need, lo = 2, 0xA0
	case c >= 0xE1 && c <= 0xEC, c == 0xEE, c == 0xEF:
		need = 2
	case c == 0xED:
		need, hi = 2, 0x9F
	case c == 0xF0:
		need, lo = 3, 0x90
	case c >= 0xF1 && c <= 0xF3:
		need = 3
	case c == 0xF4:
		need, hi = 3, 0x8F
	default:
		return 1
	}
	n := 1
	for k := 0; k < need && n < len(b); k++ {
		if b[n] < lo || b[n] > hi {
			break
		}
		lo, hi = 0x80, 0xBF
		n++
	}
	return n
}

func isDigit(r rune) bool { return unicode.Is(unicode.Nd, r) }
