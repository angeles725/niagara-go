package pycompat

import "testing"

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
