package corpusnav

import "testing"

// TestNonPositiveLimitNeverPanics: --limit/--top is a positive_int on the command line,
// but a caller passing 0 or a negative count prints no rows instead of slicing out of
// range (PR #8 follow-up).
func TestNonPositiveLimitNeverPanics(t *testing.T) {
	for _, n := range []int{0, -1} {
		c, _, _ := testCorpus(t)
		if code := c.cmdGraph(false, n, false); code != 0 {
			t.Errorf("graph top %d: exit %d", n, code)
		}
		c, _, _ = testCorpus(t)
		if code := c.cmdLintRefs(nil, true, n); code != 0 {
			t.Errorf("lint-refs limit %d: exit %d", n, code)
		}
		c, out, _ := testCorpus(t)
		if code := c.cmdSemantic("alarm", true, false, true, n); code != 0 || out.String() != "[backend: tfidf (forced by --tfidf)]\n" {
			t.Errorf("semantic limit %d: exit %d, %q", n, code, out.String())
		}
	}
}
