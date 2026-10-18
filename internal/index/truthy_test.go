package index

import (
	"encoding/json"
	"math"
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

// Values Decode never produces with no Python counterpart are falsy (an
// absent *Object is an empty dict) rather than a panic.
func TestTruthyOutsideDecode(t *testing.T) {
	for _, v := range []any{(*Object)(nil), json.Number("not a number")} {
		if Truthy(v) {
			t.Errorf("Truthy(%#v) = true, want false", v)
		}
	}
}

// Go numbers a caller computes follow Python's bool(): true unless zero.
func TestTruthyGoNumbers(t *testing.T) {
	for v, want := range map[any]bool{
		0: false, 1: true, -3: true, int64(0): false, int64(7): true,
		0.0: false, 2.5: true, math.Inf(-1): true,
	} {
		if Truthy(v) != want {
			t.Errorf("Truthy(%#v) = %v, want %v", v, !want, want)
		}
	}
	if !Truthy(math.NaN()) {
		t.Error("Truthy(NaN) = false, want true")
	}
}
