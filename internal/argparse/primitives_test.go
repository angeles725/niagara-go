package argparse

import "testing"

// textwrap and difflib semantics the help and error text depend on; every expectation was
// printed by Python 3.15 (textwrap.fill, difflib.get_close_matches). The str, repr and
// number semantics are tested in internal/pycompat.

func TestFillLikeTextwrap(t *testing.T) {
	got := fillText("Look, goof-ball -- use the -b option! x-ray self-contained well--known", 12, "  ")
	want := "  Look,\n  goof-ball\n  -- use the\n  -b option!\n  x-ray\n  self-\n  contained\n  well--\n  known"
	if got != want {
		t.Errorf("fill:\n%q\nwant\n%q", got, want)
	}
}

func TestCloseMatchLikeDifflib(t *testing.T) {
	for _, c := range []struct {
		word, want string
		choices    []string
	}{{"fats", "fast", []string{"fast", "slow", "auto"}}, {"abc", "abcy", []string{"abcx", "abcy"}},
		{"zz", "", []string{"abcx"}}} {
		if got, _ := closeMatch(c.word, c.choices); got != c.want {
			t.Errorf("close match %q = %q, want %q", c.word, got, c.want)
		}
	}
}
