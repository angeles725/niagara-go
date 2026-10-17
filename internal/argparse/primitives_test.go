package argparse

import "testing"

// textwrap semantics the help and error text depend on; every expectation was
// printed by Python 3.15 (textwrap.fill). The str, repr and number semantics are tested in
// internal/pycompat, the suggestions in internal/difflib.

func TestFillLikeTextwrap(t *testing.T) {
	got := fillText("Look, goof-ball -- use the -b option! x-ray self-contained well--known", 12, "  ")
	want := "  Look,\n  goof-ball\n  -- use the\n  -b option!\n  x-ray\n  self-\n  contained\n  well--\n  known"
	if got != want {
		t.Errorf("fill:\n%q\nwant\n%q", got, want)
	}
}
