package corpusnav

import (
	"iter"
	"strings"
	"unicode/utf8"

	"github.com/angeles725/niagara-go/internal/pycompat"
	"github.com/angeles725/niagara-go/internal/pyre"
)

// Byte scanners for the block-reference patterns of graph.py. The backtracking regexes
// (refRe, _BARE_REF_RE, _CODE_RE) spent ~80% of `graph`, `connections` and `path` on the
// 20 MB corpus; every position in them is forced, so they are written out by hand here.
// TestScannersMatchRealCorpus and TestScannersMatchAdversarial hold each scanner to its
// regex (same matches, same order, same spans and groups).
//
// The scanners work on bytes and decode a rune only where the pattern looks at one: the
// characters a match can start with ('[', 'B', 'b', '`') are ASCII, and an ASCII byte is
// always a rune of its own, valid UTF-8 or not, so a byte step is the regex's rune step.

// refMatch is one refRe match: its byte span, the group that matched (1 [Block N] /
// [Bloque N], 2 [B<N>] / [B<N> §, 3 bloqueN) and that group's digits.
type refMatch struct {
	start, end, group int
	digits            string
}

// isPyDigit is re's str-pattern \d: a Unicode decimal digit (category Nd).
func isPyDigit(r rune) bool { return pycompat.IsDecimal(r) }

// runeAt is the rune at byte i of s and its width (utf8.RuneError, 0 at the end).
func runeAt(s string, i int) (rune, int) {
	if i >= len(s) {
		return utf8.RuneError, 0
	}
	if c := s[i]; c < utf8.RuneSelf {
		return rune(c), 1
	}
	return utf8.DecodeRuneInString(s[i:])
}

// skipWhile is the end of the run of runes from byte i that satisfy ok.
func skipWhile(s string, i int, ok func(rune) bool) int {
	for i < len(s) {
		r, n := runeAt(s, i)
		if !ok(r) {
			break
		}
		i += n
	}
	return i
}

// foldPrefix reports whether s[i:] starts with the ASCII lower-case word w in any case
// ([Ll][Oo]... in the pattern).
func foldPrefix(s string, i int, w string) bool {
	if len(s)-i < len(w) {
		return false
	}
	for k := range len(w) {
		if s[i+k]|0x20 != w[k] {
			return false
		}
	}
	return true
}

// scanRefs is refRe.finditer(text):
//
//	[\[Bb] (?: (?<=\[) [Bb][Ll][Oo](?:[Cc][Kk]|[Qq][Uu][Ee])\s+\#?(\d+)\]
//	         | (?<=\[) [Bb](\d+)(?:\]|\s+§)
//	         | (?<!\w.)(?<=[Bb]) [Ll][Oo][Qq][Uu][Ee](\d+)\b )
//
// Each \s+, \#? and \d+ is followed by something it cannot match, so the greedy run is
// the only one that can succeed; \d+\b holds only when the digit run is followed by a
// non-word character (a digit is a word character).
func scanRefs(text string) iter.Seq[refMatch] {
	return func(yield func(refMatch) bool) {
		for i := 0; i < len(text); i++ {
			var m refMatch
			var ok bool
			switch text[i] {
			case '[':
				m, ok = bracketRef(text, i)
			case 'B', 'b':
				m, ok = bloqueRef(text, i)
			default:
				continue
			}
			if !ok {
				continue
			}
			if !yield(m) {
				return
			}
			i = m.end - 1 // the next search starts where the match ended
		}
	}
}

// bracketRef is the [Block N] / [Bloque #N] and [B<N>] / [B<N> § alternatives at a '['.
func bracketRef(s string, i int) (refMatch, bool) {
	j := i + 1
	if j >= len(s) || s[j]|0x20 != 'b' {
		return refMatch{}, false
	}
	j++
	if r, _ := runeAt(s, j); isPyDigit(r) { // [Bb](\d+)(?:\]|\s+§)
		d := skipWhile(s, j, isPyDigit)
		if d < len(s) && s[d] == ']' {
			return refMatch{i, d + 1, 2, s[j:d]}, true
		}
		if k := skipWhile(s, d, pyre.IsSpace); k > d && strings.HasPrefix(s[k:], "§") {
			return refMatch{i, k + len("§"), 2, s[j:d]}, true
		}
		return refMatch{}, false
	}
	switch { // [Bb][Ll][Oo](?:[Cc][Kk]|[Qq][Uu][Ee])\s+\#?(\d+)\]
	case foldPrefix(s, j, "lock"):
		j += len("lock")
	case foldPrefix(s, j, "loque"):
		j += len("loque")
	default:
		return refMatch{}, false
	}
	k := skipWhile(s, j, pyre.IsSpace)
	if k == j {
		return refMatch{}, false
	}
	if k < len(s) && s[k] == '#' {
		k++
	}
	d := skipWhile(s, k, isPyDigit)
	if d == k || d >= len(s) || s[d] != ']' {
		return refMatch{}, false
	}
	return refMatch{i, d + 1, 1, s[k:d]}, true
}

// bloqueRef is the bloqueN alternative at a 'B' or 'b': (?<!\w.) holds when the
// character before the B is not a word character (or there is none).
func bloqueRef(s string, i int) (refMatch, bool) {
	if i > 0 {
		if r, _ := utf8.DecodeLastRuneInString(s[:i]); pyre.IsWord(r) {
			return refMatch{}, false
		}
	}
	j := i + 1
	if !foldPrefix(s, j, "loque") {
		return refMatch{}, false
	}
	j += len("loque")
	d := skipWhile(s, j, isPyDigit)
	if d == j {
		return refMatch{}, false
	}
	if r, n := runeAt(s, d); n > 0 && pyre.IsWord(r) {
		return refMatch{}, false
	}
	return refMatch{i, d, 3, s[j:d]}, true
}

// stripCode is _CODE_RE.sub(" ", text), _CODE_RE = ```.*?``` | `[^`\n]*` (DOTALL): a
// fence runs to the first ``` after it; failing that, an inline span to the next
// backtick on the same line.
func stripCode(text string) string {
	if strings.IndexByte(text, '`') < 0 {
		return text
	}
	var b strings.Builder
	b.Grow(len(text))
	last := 0
	for i := 0; i < len(text); i++ {
		if text[i] != '`' {
			continue
		}
		end := -1
		if strings.HasPrefix(text[i:], "```") {
			if k := strings.Index(text[i+3:], "```"); k >= 0 {
				end = i + 3 + k + 3
			}
		}
		if end < 0 {
			if k := strings.IndexAny(text[i+1:], "`\n"); k >= 0 && text[i+1+k] == '`' {
				end = i + 1 + k + 1
			}
		}
		if end < 0 {
			continue
		}
		b.WriteString(text[last:i])
		b.WriteByte(' ')
		last = end
		i = end - 1
	}
	b.WriteString(text[last:])
	return b.String()
}

// bareRefs is _BARE_REF_RE.findall(text), (?<![\w/.\[-])B(\d{1,4})(?![\w-]): the
// lookahead fails on any longer digit run (a digit is a word character), so the run
// after the B must be 1-4 digits followed by neither a word character nor '-'.
func bareRefs(text string) iter.Seq[string] {
	return func(yield func(string) bool) {
		for i := 0; i < len(text); i++ {
			if text[i] != 'B' {
				continue
			}
			if i > 0 {
				if r, _ := utf8.DecodeLastRuneInString(text[:i]); pyre.IsWord(r) || strings.ContainsRune("/.[-", r) {
					continue
				}
			}
			j, digits := i+1, 0
			for digits <= 4 {
				r, n := runeAt(text, j)
				if n == 0 || !isPyDigit(r) {
					break
				}
				j += n
				digits++
			}
			if digits == 0 || digits > 4 {
				continue
			}
			if r, n := runeAt(text, j); n > 0 && (pyre.IsWord(r) || r == '-') {
				continue
			}
			if !yield(text[i+1 : j]) {
				return
			}
			i = j - 1
		}
	}
}
