package pycompat

import (
	"testing"
	"unicode/utf8"

	"github.com/angeles725/niagara-go/internal/textfold"
)

func isStr(pred func(string) bool) func(rune) (string, bool) {
	return func(r rune) (string, bool) { return string(r), pred(string(r)) }
}

func TestCaseMethods(t *testing.T) {
	for _, c := range load(t).Case {
		var s, lower string
		var upper bool
		decode(t, c[0], &s)
		decode(t, c[1], &upper)
		decode(t, c[2], &lower)
		if IsUpper(s) != upper {
			t.Errorf("%q: isupper = %v, want %v", s, IsUpper(s), upper)
		}
		if got := textfold.Lower(s); got != lower {
			t.Errorf("%q.lower() = %q, want %q", s, got, lower)
		}
	}
}

func TestJustify(t *testing.T) {
	widths := []int{-1, 0, 1, 2, 3, 4, 5, 6, 7, 10} // the retired gen.py's WIDTHS
	for _, c := range load(t).Just {
		var s, fill string
		var want [][2]string
		decode(t, c[0], &s)
		decode(t, c[1], &fill)
		decode(t, c[2], &want)
		f, _ := utf8.DecodeRuneInString(fill)
		for i, w := range widths {
			got := [2]string{LJust(s, w, f), RJust(s, w, f)}
			if got != want[i] {
				t.Errorf("%q width %d fill %q: ljust/rjust = %q, want %q", s, w, fill, got, want[i])
			}
		}
	}
}

func TestCut(t *testing.T) {
	for _, c := range load(t).Cut {
		var s string
		var want []string
		decode(t, c[0], &s)
		decode(t, c[1], &want)
		for i, w := range want {
			n := i - 8 // the retired gen.py's range(-8, 9)
			if got := Cut(s, n); got != w {
				t.Errorf("Cut(%q, %d) = %q, want %q", s, n, got, w)
			}
		}
	}
}
