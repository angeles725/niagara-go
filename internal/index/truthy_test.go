package index

import (
	"encoding/json"
	"strings"
	"testing"
)

// truthyCases is every value class Decode produces, with bool() of the value
// Python's json.loads gives for the same document (checked with CPython).
var truthyCases = []struct {
	doc  string
	want bool
}{
	{`null`, false}, {`true`, true}, {`false`, false},
	{`""`, false}, {`"x"`, true}, {`"0"`, true},
	{`0`, false}, {`-0`, false}, {`0.0`, false}, {`-0.0`, false}, {`0e5`, false},
	{`1e-400`, false}, // underflows to 0.0
	{`5e-324`, true},  // the smallest subnormal
	{`2`, true}, {`-1`, true},
	{`1e400`, true}, {`-1e400`, true}, // float('inf')
	{`NaN`, true}, {`Infinity`, true}, {`-Infinity`, true},
	{`1` + strings.Repeat("0", 400), true}, {`-1` + strings.Repeat("0", 400), true}, // ints beyond float64
	{`[]`, false}, {`[0]`, true}, {`[null]`, true},
	{`{}`, false}, {`{"a": null}`, true},
}

func TestTruthy(t *testing.T) {
	for _, tc := range truthyCases {
		v, err := Decode([]byte(tc.doc))
		if err != nil {
			t.Fatal(err)
		}
		if got := Truthy(v); got != tc.want {
			t.Errorf("Truthy(%.20s) = %v, want %v", tc.doc, got, tc.want)
		}
	}
}

// Values Decode never produces have no Python counterpart; they are falsy
// (an absent *Object is an empty dict) rather than a panic.
func TestTruthyOutsideDecode(t *testing.T) {
	for _, v := range []any{(*Object)(nil), json.Number("not a number"), 1, 2.5} {
		if Truthy(v) {
			t.Errorf("Truthy(%#v) = true, want false", v)
		}
	}
}
