package help

// selftest (cli/selftest.py): assert the core full-text search behaviours;
// with --recall, run the recall oracle instead (recall.go).

import (
	"bytes"
	"fmt"
	"strconv"

	"github.com/angeles725/niagara-go/internal/help/fts"
	"github.com/angeles725/niagara-go/internal/pycompat"
	"github.com/angeles725/niagara-go/internal/textfold"
)

// SelftestParams are the arguments of `selftest`.
type SelftestParams struct {
	Recall    bool
	NoSamples bool // with --recall: skip the sampled find / fuzzy rows
}

// Selftest is cmd_admin.run_selftest_command: the exit status is the
// result of the assertions (or of the recall oracle), never the misses.
func (e *Env) Selftest(p SelftestParams) error {
	if p.Recall {
		return e.Recall(!p.NoSamples)
	}
	return e.selftest()
}

// leadingCount is `int(out.split()[0]) if out.strip() and
// out.split()[0].isdigit() else 0`: the file count a ranked search prints first.
func leadingCount(out string) int {
	f := textfold.Fields(out)
	if len(f) == 0 || !isDigits(f[0]) {
		return 0
	}
	n, err := strconv.Atoi(f[0]) // our searches print the count in ASCII digits
	if err != nil {
		return 0
	}
	return n
}

// isDigits is str.isdigit.
func isDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if !pycompat.IsDigit(r) {
			return false
		}
	}
	return true
}

// selftest is selftest.run_selftest.
func (e *Env) selftest() error {
	var errs []string
	count := func(fn func(sub *Env) error) (int, error) {
		out, err := e.captured(fn)
		return leadingCount(out), err
	}
	guides := func(q string) func(sub *Env) error {
		return func(sub *Env) error { return sub.GuideSearch(FulltextParams{Query: q, Max: 10}) }
	}
	count1, err := count(guides("alarm"))
	if err != nil {
		return err
	}
	if count1 == 0 {
		errs = append(errs, "guide-search 'alarm' (single word, AND) → 0 files; expected > 0")
	} else {
		fmt.Fprintf(e.Out, "  PASS  guide-search 'alarm' → %d file(s)\n", count1)
	}
	count2, err := count(guides("alarm fault"))
	if err != nil {
		return err
	}
	if count2 == 0 {
		errs = append(errs, "guide-search 'alarm fault' (AND) → 0 files; expected > 0")
	} else {
		fmt.Fprintf(e.Out, "  PASS  guide-search 'alarm fault' (AND) → %d file(s)\n", count2)
	}
	if count1 < count2 {
		errs = append(errs, fmt.Sprintf("guide-search 'alarm' (%d) < 'alarm fault' AND (%d);"+
			" single-word should match at least as many files", count1, count2))
	} else {
		fmt.Fprintf(e.Out, "  PASS  single-word coverage: 'alarm'=%d >= 'alarm fault'=%d\n", count1, count2)
	}
	count4, err := count(func(sub *Env) error {
		return sub.DevguideSearch(FulltextParams{Query: "servlet override", Max: 10})
	})
	if err != nil {
		return err
	}
	if count4 == 0 {
		errs = append(errs, "devguide-search 'servlet override' (AND) → 0 files; expected > 0")
	} else {
		fmt.Fprintf(e.Out, "  PASS  devguide-search 'servlet override' → %d file(s)\n", count4)
	}
	if fts.TermSet("alarmAck")["alarma"] {
		errs = append(errs, "tokens: 'alarma' matched inside 'alarmAck'")
	} else {
		fmt.Fprintln(e.Out, "  PASS  'alarma' does not match inside 'alarmAck'")
	}
	fails, skipped, err := e.checkFacts()
	if err != nil {
		return err
	}
	errs = append(errs, fails...)
	if len(errs) > 0 {
		for _, m := range errs {
			fmt.Fprintf(e.Err, "  FAIL  %s\n", m)
		}
		return ExitCode(1)
	}
	if skipped > 0 {
		fmt.Fprintf(e.Out, "selftest: all assertions passed (%d skipped: their index or text source is absent or empty).\n", skipped)
	} else {
		fmt.Fprintln(e.Out, "selftest: all assertions passed.")
	}
	return ExitCode(0)
}

// captured runs fn on a copy of the Env whose stdout is a buffer (the
// _capture of selftest.py, recall._captured); misses and stderr go where
// they normally do.
func (e *Env) captured(fn func(sub *Env) error) (string, error) {
	var buf bytes.Buffer
	sub := *e
	sub.Out = &buf
	err := fn(&sub)
	return buf.String(), err
}
