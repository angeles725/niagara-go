// Package license is the licensing-call scan of module-navigator
// license_features.py (and the java_text.py helpers it uses): which
// vendor:feature pairs the corpus asks Sys.getLicenseManager() for, and where.
//
// The Python scanner is built on `re`; Go's regexp differs in \w, \s and \b
// (ASCII only), so each pattern is a small hand-written matcher with Python's
// Unicode classes (pycompat.IsWordRune, pycompat.IsSpace) and the same
// leftmost-first backtracking result. Texts are UTF-8 with byte offsets; the
// one code-point window of _receiver is measured in runes.
package license

import (
	"strings"
	"unicode/utf8"

	"github.com/angeles725/niagara-go/internal/pycompat"
)

// span is a [start, end) byte range.
type span struct{ start, end int }

// commentSpans is java_text.comment_spans: every // and /* */ comment,
// skipping string and char literals (which end at a newline).
func commentSpans(text string) []span {
	var spans []span
	n := len(text)
	for i := 0; i < n; {
		c := text[i]
		switch {
		case c == '"' || c == '\'':
			j := i + 1
			for j < n && text[j] != c && text[j] != '\n' {
				if text[j] == '\\' {
					j += 2
				} else {
					j++
				}
			}
			i = j + 1
		case strings.HasPrefix(text[i:], "//"):
			j := strings.IndexByte(text[i:], '\n')
			if j < 0 {
				j = n
			} else {
				j += i
			}
			spans = append(spans, span{i, j})
			i = j
		case strings.HasPrefix(text[i:], "/*"):
			j := strings.Index(text[i+2:], "*/")
			if j < 0 {
				j = n
			} else {
				j += i + 4
			}
			spans = append(spans, span{i, j})
			i = j
		default:
			i++
		}
	}
	return spans
}

func inSpans(pos int, spans []span) bool {
	for _, s := range spans {
		if s.start <= pos && pos < s.end {
			return true
		}
	}
	return false
}

// callArgs is java_text.call_args: the stripped arguments of the call whose
// '(' is at openParen, [] for none, ok=false when unbalanced.
func callArgs(text string, openParen int) (args []string, ok bool) {
	depth := 0
	var buf strings.Builder
	for i := openParen + 1; i < len(text); {
		c := text[i]
		if c == '"' {
			j := i + 1
			for j < len(text) && text[j] != '"' {
				if text[j] == '\\' {
					j += 2
				} else {
					j++
				}
			}
			buf.WriteString(text[i:min(j+1, len(text))])
			i = j + 1
			continue
		}
		switch c {
		case '(', '[', '{':
			depth++
		case ')', ']', '}':
			if depth == 0 {
				args = append(args, pycompat.Strip(buf.String()))
				if len(args) == 1 && args[0] == "" {
					return []string{}, true
				}
				return args, true
			}
			depth--
		case ',':
			if depth == 0 {
				args = append(args, pycompat.Strip(buf.String()))
				buf.Reset()
				i++
				continue
			}
		}
		buf.WriteByte(c)
		i++
	}
	return nil, false
}

// isWord is re's \w; isIdent is [\w$].
func isWord(r rune) bool  { return pycompat.IsWordRune(r) }
func isIdent(r rune) bool { return r == '$' || isWord(r) }

func runeAt(s string, i int) (rune, int) {
	if i < 0 || i >= len(s) {
		return utf8.RuneError, 0
	}
	return utf8.DecodeRuneInString(s[i:])
}

// wordAt and wordBefore tell whether the character at / before byte i is a
// word character (false outside the text).
func wordAt(s string, i int) bool {
	r, n := runeAt(s, i)
	return n > 0 && isWord(r)
}

func wordBefore(s string, i int) bool {
	if i <= 0 {
		return false
	}
	r, _ := utf8.DecodeLastRuneInString(s[:i])
	return isWord(r)
}

// boundary is re's \b at byte i.
func boundary(s string, i int) bool { return wordBefore(s, i) != wordAt(s, i) }

// skipSpace is \s* from byte i.
func skipSpace(s string, i int) int {
	for i < len(s) {
		r, n := utf8.DecodeRuneInString(s[i:])
		if !pycompat.IsSpace(r) {
			break
		}
		i += n
	}
	return i
}

// skipRun is [class]* from byte i.
func skipRun(s string, i int, class func(rune) bool) int {
	for i < len(s) {
		r, n := utf8.DecodeRuneInString(s[i:])
		if !class(r) {
			break
		}
		i += n
	}
	return i
}

// lit matches the literal word at byte i; the end is -1 when it does not.
func lit(s string, i int, word string) int {
	if i >= 0 && strings.HasPrefix(s[i:], word) {
		return i + len(word)
	}
	return -1
}

// stringBody is "((?:[^"\\]|\\.)*)" at byte i: the literal's content and the
// offset after its closing quote (`.` does not match a newline).
func stringBody(s string, i int) (content string, end int, ok bool) {
	if i < 0 || i >= len(s) || s[i] != '"' {
		return "", -1, false
	}
	for j := i + 1; j < len(s); {
		switch s[j] {
		case '"':
			return s[i+1 : j], j + 1, true
		case '\\':
			r, n := runeAt(s, j+1)
			if n == 0 || r == '\n' {
				return "", -1, false
			}
			j += 1 + n
		default:
			_, n := utf8.DecodeRuneInString(s[j:])
			j += n
		}
	}
	return "", -1, false
}

// occurrences calls f with every offset of word in s, in order, until f
// returns true.
func occurrences(s, word string, f func(i int) bool) {
	for i := 0; i <= len(s); { // an empty word matches at every offset, len(s) included
		k := strings.Index(s[i:], word)
		if k < 0 {
			return
		}
		if f(i + k) {
			return
		}
		i += k + 1
	}
}
