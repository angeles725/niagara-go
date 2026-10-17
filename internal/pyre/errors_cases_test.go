package pyre

import (
	"errors"
	"testing"
)

// TestErrors compares every invalid pattern with the re.error text CPython raised
// (testdata/gen.py), message and position, through the public Compile: the error is
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
