package pyre

import (
	"errors"
	"testing"
)

// TestErrors compares every invalid pattern with the re.error text CPython raised
// (the retired testdata/gen.py), message and position, through the public Compile: the error is
// an *Error and no pattern of the fixture compiles.
func TestErrors(t *testing.T) {
	var f errorFixture
	loadJSON(t, "testdata/errors.json", &f)
	if len(f.Cases) < 100 {
		t.Fatalf("only %d cases in testdata/errors.json", len(f.Cases))
	}
	for _, c := range f.Cases {
		re, err := Compile(c.Pattern, c.Flags)
		var perr *Error
		if re != nil || !errors.As(err, &perr) || err.Error() != c.Error {
			t.Errorf("Compile(%q, %d) = %v, error %v, want *Error %q", c.Pattern, c.Flags, re, err, c.Error)
		}
	}
	if _, _, err := validate("(?P<n>a)(?(n)b|c)(?<=\\1)", 0); err != nil {
		t.Errorf("valid pattern rejected: %v", err)
	}
}

// TestFlagArgumentErrors is _parser.fix_flags on the flags argument (CPython's ValueError
// text, no position): LOCALE never applies to a str pattern, ASCII excludes UNICODE.
func TestFlagArgumentErrors(t *testing.T) {
	for flags, want := range map[Flags]string{
		LOCALE:                       "cannot use LOCALE flag with a str pattern",
		LOCALE | ASCII:               "cannot use LOCALE flag with a str pattern",
		ASCII | UNICODE:              "ASCII and UNICODE flags are incompatible",
		ASCII | UNICODE | IGNORECASE: "ASCII and UNICODE flags are incompatible",
	} {
		if _, err := Compile("a", flags); err == nil || err.Error() != want {
			t.Errorf("Compile(%q, %d) error = %v, want %q", "a", flags, err, want)
		}
	}
	if _, err := Compile("a", UNICODE|IGNORECASE); err != nil {
		t.Errorf("UNICODE|IGNORECASE rejected: %v", err)
	}
}

// TestLookaroundEdges pins three lookaround branches of parseGroup with CPython 3.15's
// results: "(?<" ending the pattern and an unclosed lookahead raise re.error, and an empty
// negative lookaround compiles to a pattern that never matches, not even the empty string
// (sre_parse turns "(?!)" into FAILURE).
func TestLookaroundEdges(t *testing.T) {
	for pattern, want := range map[string]string{
		"(?<":  "unexpected end of pattern at position 3",
		"(?=a": "missing ), unterminated subpattern at position 0",
	} {
		if _, err := Compile(pattern, 0); err == nil || err.Error() != want {
			t.Errorf("Compile(%q) error = %v, want %q", pattern, err, want)
		}
	}
	for _, pattern := range []string{"(?!)", "(?<!)", "a(?!)|b"} {
		re, err := Compile(pattern, 0)
		if err != nil {
			t.Fatalf("Compile(%q): %v", pattern, err)
		}
		for _, s := range []string{"", "a", "x"} {
			if m := re.Search(s); m != nil {
				t.Errorf("Compile(%q).Search(%q) matched at %d, want no match", pattern, s, m.Start())
			}
		}
	}
	if m := MustCompile("a(?!)|b", 0).Search("ab"); m == nil || m.Group(0) != "b" {
		t.Errorf(`"a(?!)|b" on "ab" = %v, want the match "b"`, m)
	}
	for pattern, want := range map[string]kind{"(?!)": kFailure, "(?<!)": kFailure, "(?!a)": kAssertNot} {
		if seq, _, err := parse(pattern, 0); err != nil || len(seq) != 1 || seq[0].kind != want {
			t.Errorf("parse(%q) = %v, %v; want one node of kind %v", pattern, seq, err, want)
		}
	}
}
