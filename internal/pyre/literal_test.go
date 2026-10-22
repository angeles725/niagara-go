package pyre

import (
	"strings"
	"testing"
)

// literalSubjects mixes ASCII, case variants, the Unicode folds of ASCII
// letters (Kelvin sign, long s, dotless i), invalid UTF-8 and newlines.
var literalSubjects = []string{
	"", "a", "aaaa", "abc", "ABC", "xAbCx", "checkFeature(x)", "CHECKFEATURE", "checkfeatur",
	"BStatusNumeric", "bstatusnumeric", "line one\nline two\n", "tab\there", "a-b_c/d",
	"K", "kK", "ſs", "ıi", "café", "\xff\xfeabc", "ab\xffc", "\x00ab",
	"overlap ababab", "#comment", "x y", " leading", "trailing ",
}

var literalPatterns = []string{
	"", "a", "aa", "abc", "ABC", "b", "k", "K", "s", "S", "i", "I", "checkFeature", "BStatusNumeric",
	"line two", "\n", "aba", "-", "a-b", "/", "#", " ", "x y", "\t", "_c/", "e(", "a.c", "^a",
}

var literalFlags = []Flags{0, IGNORECASE, ASCII, ASCII | IGNORECASE, MULTILINE, DOTALL, VERBOSE, IGNORECASE | VERBOSE}

func spans(m *Match) [2]int {
	if m == nil {
		return [2]int{-2, -2}
	}
	s, e := m.Span(0)
	return [2]int{s, e}
}

// checkLiteral compares the string entry points (which take the literal fast
// path) with the regexp2 search over runes, which never does.
func checkLiteral(t *testing.T, pattern string, flags Flags, subject string) {
	t.Helper()
	re, err := Compile(pattern, flags)
	if err != nil {
		return
	}
	slow := re.SearchRunes([]rune(subject), 0)
	if got, want := spans(re.Search(subject)), spans(slow); got != want {
		t.Errorf("Search(%q, %d) on %q: span %v, regexp2 %v", pattern, flags, subject, got, want)
	}
	if got := re.MatchString(subject); got != (slow != nil) {
		t.Errorf("MatchString(%q, %d) on %q: %v", pattern, flags, subject, got)
	}
	if !re.MayMatch(subject) && slow != nil {
		t.Errorf("MayMatch(%q, %d) on %q is false, regexp2 matches", pattern, flags, subject)
	}
	g := 0
	if re.Groups() > 0 {
		g = 1
	}
	var want []string
	for _, m := range re.FindIter([]rune(subject)) {
		want = append(want, m.Group(g))
	}
	if got := re.FindAll(subject); strings.Join(got, "\x01") != strings.Join(want, "\x01") || len(got) != len(want) {
		t.Errorf("FindAll(%q, %d) on %q: %q, regexp2 %q", pattern, flags, subject, got, want)
	}
}

// TestLiteralFastPathDifferential pins the fast path to regexp2 on every
// pattern, flag set and subject of the table.
func TestLiteralFastPathDifferential(t *testing.T) {
	for _, p := range literalPatterns {
		for _, f := range literalFlags {
			for _, s := range literalSubjects {
				checkLiteral(t, p, f, s)
			}
		}
	}
}

// TestLiteralFastPathEngaged pins which patterns take the fast path and that
// it rejects what it can decide: a fast path that is never taken is no speedup.
func TestLiteralFastPathEngaged(t *testing.T) {
	cases := []struct {
		pattern string
		flags   Flags
		literal bool
	}{
		{"checkFeature", 0, true},
		{"checkFeature", IGNORECASE, true},
		{"a-b /#_", 0, true},
		{"", 0, true},
		{"a.c", 0, false},
		{"a\\.c", 0, false},
		{"x y", VERBOSE, false},
		{"café", 0, false},
		{"(a)", 0, false},
	}
	for _, c := range cases {
		re := MustCompile(c.pattern, c.flags)
		if got := re.lit != nil; got != c.literal {
			t.Errorf("%q flags %d: literal fast path %v, want %v", c.pattern, c.flags, got, c.literal)
		}
	}
	if MustCompile("checkFeature", 0).MayMatch("checkfeature") {
		t.Error("case-sensitive literal: MayMatch must reject a case variant")
	}
	if !MustCompile("checkFeature", IGNORECASE).MayMatch("x CHECKFEATURE y") {
		t.Error("IGNORECASE literal: MayMatch must accept an ASCII case variant")
	}
	if MustCompile("checkFeature", IGNORECASE).MayMatch("x checkFeatur y") {
		t.Error("IGNORECASE literal: MayMatch must reject an ASCII subject without it")
	}
	if !MustCompile("k", IGNORECASE).MayMatch("K") {
		t.Error("IGNORECASE literal: a non-ASCII subject must fall back to regexp2")
	}
}

// FuzzLiteralFastPath is the differential check on arbitrary literal
// patterns and subjects.
func FuzzLiteralFastPath(f *testing.F) {
	for _, s := range literalSubjects {
		f.Add("ab", s, false)
		f.Add("K", s, true)
	}
	f.Fuzz(func(t *testing.T, pattern, subject string, fold bool) {
		flags := Flags(0)
		if fold {
			flags = IGNORECASE
		}
		checkLiteral(t, pattern, flags, subject)
	})
}
