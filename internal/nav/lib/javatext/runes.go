package javatext

import (
	"unicode/utf8"

	"github.com/angeles725/niagara-go/internal/pycompat"
	"github.com/angeles725/niagara-go/internal/textfold"
)

// RuneAt is the rune starting at byte i of s and its width; (RuneError, 0)
// outside the text.
func RuneAt(s string, i int) (rune, int) {
	if i < 0 || i >= len(s) {
		return utf8.RuneError, 0
	}
	return utf8.DecodeRuneInString(s[i:])
}

// WordAt and WordBefore tell whether the character at / just before byte i is
// a word character, re's \w (false outside the text).
func WordAt(s string, i int) bool {
	r, n := RuneAt(s, i)
	return n > 0 && pycompat.IsWordRune(r)
}

func WordBefore(s string, i int) bool {
	if i <= 0 {
		return false
	}
	r, _ := utf8.DecodeLastRuneInString(s[:i])
	return pycompat.IsWordRune(r)
}

// SkipSpace is re's \s* from byte i: the offset after the run.
func SkipSpace(s string, i int) int {
	for i < len(s) {
		r, n := utf8.DecodeRuneInString(s[i:])
		if !textfold.IsSpace(r) {
			break
		}
		i += n
	}
	return i
}
