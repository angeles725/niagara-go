package pyre

import "testing"

// TestErrors compares every invalid pattern with the re.error text CPython raised
// (testdata/gen.py), message and position.
func TestErrors(t *testing.T) {
	var f errorFixture
	loadJSON(t, "testdata/errors.json", &f)
	for _, c := range f.Cases {
		_, _, err := validate(c.Pattern, c.Flags)
		if err == nil || err.Error() != c.Error {
			t.Errorf("validate(%q, %d) error = %v, want %q", c.Pattern, c.Flags, err, c.Error)
		}
	}
	if _, _, err := validate("(?P<n>a)(?(n)b|c)(?<=\\1)", 0); err != nil {
		t.Errorf("valid pattern rejected: %v", err)
	}
}
