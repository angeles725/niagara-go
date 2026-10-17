package modulefind

import "testing"

// TestNegativeLimitNeverPanics: --limit is a positive_int on the command line and 0
// means "not given"; a caller passing a negative count gets every row, never a slice
// out of range (PR #8 follow-up).
func TestNegativeLimitNeverPanics(t *testing.T) {
	all, _ := runCmd(t, opts{}, func(m *module, o *opts) { cmdOrds(m, o, nil) })
	got, stderr := runCmd(t, opts{limit: -1}, func(m *module, o *opts) { cmdOrds(m, o, nil) })
	if got != all || stderr != "" {
		t.Errorf("limit -1: %q / %q, want %q", got, stderr, all)
	}
}
