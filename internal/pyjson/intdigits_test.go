package pyjson

import (
	"errors"
	"strings"
	"testing"
)

// TestLoadsLimitsIntDigits: CPython 3.15 json.loads raises ValueError "Exceeds the limit
// (4300 digits) ..." at the first int literal past 4300 digits, before any later syntax
// error; the sign does not count and float literals are not limited.
func TestLoadsLimitsIntDigits(t *testing.T) {
	ones := func(n int) string { return strings.Repeat("1", n) }
	msg := func(n string) string {
		return "Exceeds the limit (4300 digits) for integer string conversion: value has " + n +
			" digits; use sys.set_int_max_str_digits() to increase the limit"
	}
	for _, tc := range []struct{ name, doc, err string }{
		{"4300 digits", ones(4300), ""},
		{"signed 4300 digits", "-" + ones(4300), ""},
		{"float mantissa", ones(4301) + ".5", ""},
		{"float exponent form", ones(4301) + "e1", ""},
		{"4301 digits", ones(4301), msg("4301")},
		{"signed 4301 digits", "-" + ones(4301), msg("4301")},
		{"inside an array", "[" + ones(5000) + "]", msg("5000")},
		{"before a syntax error", "[1" + strings.Repeat("0", 4300) + ", x", msg("4301")},
	} {
		_, err := Loads([]rune(tc.doc))
		var de *IntDigitsError
		switch {
		case tc.err == "" && err != nil:
			t.Errorf("%s: unexpected %v", tc.name, err)
		case tc.err != "" && (!errors.As(err, &de) || err.Error() != tc.err):
			t.Errorf("%s: got %v, want IntDigitsError %q", tc.name, err, tc.err)
		}
	}
}
