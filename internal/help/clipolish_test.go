package help

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/angeles725/niagara-go/internal/help/fts/ftsfixture"
)

// polishCheckout is the full-text fixture checkout the CLI polish cases run on.
func polishCheckout(t *testing.T) string {
	t.Helper()
	testdata, err := filepath.Abs("testdata")
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	return ftsfixture.Checkout(t, testdata, false)
}

// A negative -n is a usage error (exit 2) on every command that takes one;
// before, find -n -5 printed "... and N more files" and guide-search counted
// from the end.
func TestNegativeMaxIsUsageError(t *testing.T) {
	dir := polishCheckout(t)
	for _, args := range [][]string{
		{"find", "alarm", "-n", "-5"},
		{"guide-search", "alarm", "-n", "-1"},
		{"devguide-search", "alarm", "-n", "-1"},
		{"source-grep", "alarm", "-n", "-3"},
	} {
		out, errs, code := run(t, dir, append([]string{"--niagara-version", "1.0"}, args...)...)
		if code != 2 || out != "" || !strings.Contains(errs, "must be 0 or more, not -") {
			t.Errorf("%q: exit %d stdout %q stderr %q; want 2, no stdout, a -n usage error", args, code, out, errs)
		}
	}
}

// class "" is invalid input like find "" (exit 2), not a miss (exit 1).
func TestClassEmptyNameIsUsageError(t *testing.T) {
	dir := polishCheckout(t)
	for _, name := range []string{"", "   "} {
		out, errs, code := run(t, dir, "--niagara-version", "1.0", "class", name)
		if code != 2 || out != "" || !strings.HasPrefix(errs, "ERROR: ") {
			t.Errorf("class %q: exit %d stdout %q stderr %q; want 2, no stdout, an ERROR line", name, code, out, errs)
		}
	}
}

// A very long query is searched whole but echoed cut, so a pasted page does
// not come back three times in the miss block.
func TestLongQueryEchoIsCut(t *testing.T) {
	dir := polishCheckout(t)
	query := strings.TrimSpace(strings.Repeat("zzqq wwvv ", 1500))
	out, _, code := run(t, dir, "--niagara-version", "1.0", "find", query)
	if code != 1 {
		t.Fatalf("exit %d, want 1 (a miss)", code)
	}
	if strings.Contains(out, query) || len(out) > 2000 {
		t.Errorf("the miss block echoes the whole %d-byte query (%d bytes):\n%.600s", len(query), len(out), out)
	}
	if !strings.Contains(out, "No match for 'zzqq wwvv zzqq") || !strings.Contains(out, "more characters") {
		t.Errorf("the cut echo does not start with the query or say it was cut:\n%.600s", out)
	}
}

// A Spanish glossary word searched as its English terms is noted once on
// stderr; the ranking is unchanged.
func TestGlossaryRewriteNote(t *testing.T) {
	dir := polishCheckout(t)
	out, errs, code := run(t, dir, "--niagara-version", "1.0", "find", "como funciona el ack de alarmas")
	if code != 0 || !strings.HasPrefix(out, "6 files matching") {
		t.Fatalf("exit %d stdout %.200q", code, out)
	}
	if strings.Count(errs, "\n") != 1 || !strings.Contains(errs, "alarmas -> alarm") {
		t.Errorf("stderr %q, want one note naming alarmas -> alarm", errs)
	}
	if _, errs, _ := run(t, dir, "--niagara-version", "1.0", "find", "alarm ack"); errs != "" {
		t.Errorf("an English query printed a note: %q", errs)
	}
}
