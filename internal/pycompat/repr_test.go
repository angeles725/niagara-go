package pycompat

import (
	"encoding/json"
	"testing"
)

// point knows its own Python repr and also has a Go String; Repr must prefer PyRepr.
type point struct{ x, y int }

func (p point) PyRepr() string { return "Point(x=" + Repr(p.x) + ", y=" + Repr(p.y) + ")" }
func (p point) String() string { return "go-string" }

// ref has PyRepr on the pointer receiver only.
type ref struct{ name string }

func (r *ref) PyRepr() string { return "<ref " + StrRepr(r.name) + ">" }

func TestReprHonoursReprer(t *testing.T) {
	for _, tc := range []struct {
		name string
		fn   func(any) string
		v    any
		want string
	}{
		{"value receiver wins over String", Repr, point{1, -2}, "Point(x=1, y=-2)"},
		{"pointer receiver", Repr, &ref{"a'b"}, `<ref "a'b">`},
		{"inside a list", Repr, []any{point{0, 0}, "s", nil}, "[Point(x=0, y=0), 's', None]"},
		{"inside a one-tuple", Repr, Tuple{&ref{"t"}}, "(<ref 't'>,)"},
		{"Str of a Reprer", Str, point{3, 4}, "Point(x=3, y=4)"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.fn(tc.v); got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
	var _ Reprer = point{}
}

// object is an insertion-ordered JSON object, the shape of index.Object.
type object struct {
	keys []string
	vals []any
}

func (o object) Keys() []string { return o.keys }
func (o object) Values() []any  { return o.vals }

// A json.Number is repr() of the int or float json.loads gives it; an OrderedMap is a dict.
func TestReprJSONValues(t *testing.T) {
	for _, tc := range []struct {
		name string
		fn   func(any) string
		v    any
		want string
	}{
		{"int", Repr, json.Number("42"), "42"},
		{"negative zero int", Repr, json.Number("-0"), "0"},
		{"int beyond int64", Repr, json.Number("123456789012345678901234567890"), "123456789012345678901234567890"},
		{"float", Repr, json.Number("2.50"), "2.5"},
		{"exponent float", Repr, json.Number("1e5"), "100000.0"},
		{"overflowing float", Repr, json.Number("1e400"), "inf"},
		{"negative overflowing float", Repr, json.Number("-1e400"), "-inf"},
		{"NaN literal", Repr, json.Number("NaN"), "nan"},
		{"Infinity literal", Repr, json.Number("-Infinity"), "-inf"},
		{"Str of a number", Str, json.Number("7"), "7"},
		{"empty dict", Repr, object{}, "{}"},
		{"dict in insertion order", Repr,
			object{[]string{"b", "a'"}, []any{[]any{json.Number("1"), nil, true}, object{[]string{"c"}, []any{"x"}}}},
			`{'b': [1, None, True], "a'": {'c': 'x'}}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.fn(tc.v); got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
	var _ OrderedMap = object{}
}
