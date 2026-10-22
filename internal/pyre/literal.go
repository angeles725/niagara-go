package pyre

import "strings"

// The literal fast path. A pattern made only of ASCII characters that are not
// regex syntax, compiled without VERBOSE, matches exactly where its text
// occurs, so a subject that does not hold the text cannot match and the
// backtracking engine need not run. The fast path only rejects: a subject
// that holds the text is still searched by regexp2, so every match, span and
// group is the engine's. Under IGNORECASE the text is compared with ASCII case
// folding, and only on ASCII subjects: a non-ASCII subject (where Unicode
// folds such as the Kelvin sign for k apply) always goes to regexp2.

// literal is the fast-path form of a literal pattern.
type literal struct {
	text string // the pattern; lower-cased when fold
	fold bool   // IGNORECASE
}

// regexSyntax are the characters that make a pattern not a literal.
const regexSyntax = `\.^$*+?{}[]|()`

// literalOf is the fast-path form of pattern under flags, nil when the
// pattern is not an ASCII literal or VERBOSE gives spaces and # a meaning.
func literalOf(pattern string, flags Flags) *literal {
	if flags&VERBOSE != 0 || !isASCII(pattern) || strings.ContainsAny(pattern, regexSyntax) {
		return nil
	}
	if flags&IGNORECASE != 0 {
		return &literal{text: asciiLower(pattern), fold: true}
	}
	return &literal{text: pattern}
}

// MayMatch reports whether a search of s, or of any substring of s, can find
// a match: false only when the literal fast path proves it cannot; true for
// every pattern that is not a literal.
func (r *Regexp) MayMatch(s string) bool {
	l := r.lit
	switch {
	case l == nil:
		return true
	case !l.fold:
		// An ASCII text matches the same in the bytes as in the code points
		// regexp2 sees: an invalid byte decodes to U+FFFD, never to ASCII.
		return strings.Contains(s, l.text)
	case !isASCII(s):
		return true
	default:
		return containsFold(s, l.text)
	}
}

// containsFold is strings.Contains with ASCII case folding; lower is already
// lower-cased and s is ASCII.
func containsFold(s, lower string) bool {
	n := len(lower)
	if n == 0 {
		return true
	}
	first, firstUp := lower[0], upper(lower[0])
	for i := 0; i+n <= len(s); i++ {
		if c := s[i]; c != first && c != firstUp {
			continue
		}
		j := 1
		for j < n && toLower(s[i+j]) == lower[j] {
			j++
		}
		if j == n {
			return true
		}
	}
	return false
}

func isASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] >= 0x80 {
			return false
		}
	}
	return true
}

func toLower(c byte) byte {
	if 'A' <= c && c <= 'Z' {
		return c + 'a' - 'A'
	}
	return c
}

func upper(c byte) byte {
	if 'a' <= c && c <= 'z' {
		return c - ('a' - 'A')
	}
	return c
}

func asciiLower(s string) string {
	b := []byte(s)
	for i, c := range b {
		b[i] = toLower(c)
	}
	return string(b)
}
