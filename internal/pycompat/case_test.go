package pycompat

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func isStr(pred func(string) bool) func(rune) (string, bool) {
	return func(r rune) (string, bool) { return string(r), pred(string(r)) }
}

func mapStr(f func(string) string) func(rune) (string, bool) {
	return func(r rune) (string, bool) { return f(string(r)), true }
}

// caseContext mirrors gen.py case_context: the multi-character paths around ch.
func caseContext(ch string) string {
	b := func(v bool) string {
		if v {
			return "1"
		}
		return "0"
	}
	const sigma = "Σ"
	flags := b(IsUpper(ch+"A")) + b(IsLower(ch+"a")) + b(IsTitle(ch+"ab")) + b(IsTitle("A"+ch))
	return strings.Join([]string{flags, Title("a" + ch + "b"), SwapCase("A" + ch + sigma), Capitalize("x" + ch),
		Title(ch + sigma + "a"), Capitalize("A" + sigma + ch)}, "|")
}

func TestCaseMethods(t *testing.T) {
	for _, c := range load(t).Case {
		var s string
		var upper, lower, title bool
		var maps [5]string
		decode(t, c[0], &s)
		decode(t, c[1], &upper)
		decode(t, c[2], &lower)
		decode(t, c[3], &title)
		for i := range maps {
			decode(t, c[4+i], &maps[i])
		}
		if IsUpper(s) != upper || IsLower(s) != lower || IsTitle(s) != title {
			t.Errorf("%q: isupper/islower/istitle = %v %v %v, want %v %v %v",
				s, IsUpper(s), IsLower(s), IsTitle(s), upper, lower, title)
		}
		got := [5]string{Upper(s), Lower(s), Title(s), Capitalize(s), SwapCase(s)}
		for i, name := range []string{"upper", "lower", "title", "capitalize", "swapcase"} {
			if got[i] != maps[i] {
				t.Errorf("%q.%s() = %q, want %q", s, name, got[i], maps[i])
			}
		}
	}
}

func TestJustify(t *testing.T) {
	widths := []int{-1, 0, 1, 2, 3, 4, 5, 6, 7, 10} // gen.py WIDTHS
	for _, c := range load(t).Just {
		var s, fill string
		var want [][3]string
		decode(t, c[0], &s)
		decode(t, c[1], &fill)
		decode(t, c[2], &want)
		f, _ := utf8.DecodeRuneInString(fill)
		for i, w := range widths {
			got := [3]string{LJust(s, w, f), RJust(s, w, f), Center(s, w, f)}
			if got != want[i] {
				t.Errorf("%q width %d fill %q: ljust/rjust/center = %q, want %q", s, w, fill, got, want[i])
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
			n := i - 8 // gen.py range(-8, 9)
			if got := Cut(s, n); got != w {
				t.Errorf("Cut(%q, %d) = %q, want %q", s, n, got, w)
			}
		}
	}
}
