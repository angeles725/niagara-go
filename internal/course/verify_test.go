package course

import (
	"bytes"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/angeles725/niagara-go/internal/cli"
)

func TestDecidePrediction(t *testing.T) {
	r := func(id, verdict, ev string, notes ...string) linkedResult {
		return linkedResult{Result: Result{Check: &Check{ID: id}, Verdict: verdict, Evidence: ev}, Notes: notes}
	}
	pass, pass2 := r("C1.1", verdictPass, "found 1"), r("C1.2", verdictPass, "x = 1 (stored)")
	part := r("C1.5", verdictPass, "y stored", "the console line")
	part2 := r("C1.6", verdictPass, "w stored", "the console line", "the log")
	fail := r("C1.3", verdictFail, "found 0, want at least 1")
	skip := r("C1.4", verdictSkip, "no --install DIR")
	cases := []struct {
		results         []linkedResult
		verdict, reason string
	}{
		{nil, predUndecided, "no check covers it"},
		{[]linkedResult{pass, pass2}, predConfirmed, "C1.1 PASS: found 1; C1.2 PASS: x = 1 (stored)"},
		{[]linkedResult{pass, fail, skip}, predContradicted, "C1.3 FAIL: found 0, want at least 1"},
		{[]linkedResult{pass, skip}, predUndecided, "C1.4 SKIP: no --install DIR"},
		{[]linkedResult{skip}, predUndecided, "C1.4 SKIP: no --install DIR"},
		{[]linkedResult{part, fail}, predContradicted, "C1.3 FAIL: found 0, want at least 1"},
		{[]linkedResult{part, skip}, predUndecided, "C1.4 SKIP: no --install DIR"},
		// Every passing check observes only the stored part: PARTIAL, with what remains.
		{[]linkedResult{part, part2}, predPartial,
			"C1.5 PASS: y stored; C1.6 PASS: w stored; not observed: the console line; the log"},
		// One check that observes the whole answer is enough.
		{[]linkedResult{part, pass}, predConfirmed, "C1.5 PASS: y stored; C1.1 PASS: found 1"},
	}
	for _, c := range cases {
		if v, why := decide(c.results); v != c.verdict || why != c.reason {
			t.Errorf("decide(%d results) = %s %q, want %s %q", len(c.results), v, why, c.verdict, c.reason)
		}
	}
	if got := suggestion("L1.1", "2026-10-17", []linkedResult{pass, pass2}); got !=
		"L1.1: *(verified live 2026-10-17)* evidence: C1.1 found 1; C1.2 x = 1 (stored)" {
		t.Errorf("suggestion %q", got)
	}
}

// itemLinks reads a note under the key that matched (a covered ID of a range item), not under
// the item's ID, and links a check once however many of its keys reach the item.
func TestItemLinks(t *testing.T) {
	c, problems := load("testdata/root") // holds the range item L2.2–L2.4
	if c == nil {
		t.Fatalf("%v", problems)
	}
	spec := []Check{
		{ID: "C2.1", Keys: []string{"L2.3"}, Unobserved: map[string]string{"L2.3": "a view"}},
		{ID: "C2.2", Keys: []string{"L2.2", "L2.4"}, Unobserved: map[string]string{"L2.4": "a log"}},
		{ID: "C2.3", Keys: []string{"L2.2", "L2.9"}},
	}
	ls := itemLinks(c, spec)[c.lookup("L2.3")]
	if len(ls) != 3 || ls[0].Check.ID != "C2.1" || !slices.Equal(ls[0].Notes, []string{"a view"}) ||
		ls[1].Check.ID != "C2.2" || !slices.Equal(ls[1].Notes, []string{"a log"}) || ls[2].Check.ID != "C2.3" || ls[2].Notes != nil {
		t.Errorf("links of L2.2–L2.4: %+v", ls)
	}
}

// A key reaches a range item through a covered ID: its note still applies (PARTIAL, no
// suggestion), and a check whose several keys reach one item is linked once.
func TestVerifyPredictionsThroughRange(t *testing.T) {
	c, problems := load("testdata/predict-root")
	if c == nil {
		t.Fatalf("%v", problems)
	}
	preds, st, err := verifyPredictions(c, goodStation, fixtureInstall)
	if err != nil {
		t.Fatal(err)
	}
	defer st.close()
	got := map[string]prediction{}
	for _, p := range preds {
		got[p.Item.ID] = p
	}
	if p := got["L4.15–L4.16"]; p.Verdict != predPartial || !strings.Contains(p.Reason, "not observed: the outputs at 72.5") {
		t.Errorf("range L4.15–L4.16: %s %q, want PARTIAL with the C4.6 note", p.Verdict, p.Reason)
	}
	if p := got["L10.6–L10.7"]; p.Verdict != predConfirmed || len(p.Results) != 1 {
		t.Errorf("range L10.6–L10.7: %s with %d results, want CONFIRMED from C10.7 once", p.Verdict, len(p.Results))
	}
}

// Without --date the suggestion lines carry today's date from the injected clock.
func TestVerifyPredictionsDefaultDate(t *testing.T) {
	defer func(f func() time.Time) { now = f }(now)
	now = func() time.Time { return time.Date(2031, 2, 3, 23, 30, 0, 0, time.Local) }
	var out, errb bytes.Buffer
	code := Main([]string{"--root", "testdata/predict-root", "verify-predictions", "--station", goodStation,
		"--install", fixtureInstall}, cli.Stdio{In: strings.NewReader(""), Out: &out, Err: &errb})
	if code != 0 || !strings.Contains(out.String(), "L4.13: *(verified live 2031-02-03)*") {
		t.Errorf("exit %d, stdout:\n%s\nstderr: %s", code, out.String(), errb.String())
	}
}

// TestVerifyPredictionsTrainingStation runs verify-predictions on the live TrainingStation
// against the real answer key; it pins only what the live check test pins: C3.4 is the one
// FAIL, so no predicted item may be CONTRADICTED (L3.7, its key, is not predicted).
func TestVerifyPredictionsTrainingStation(t *testing.T) {
	path, root := os.Getenv("COURSE_TRAINING_STATION"), os.Getenv(RootEnv)
	if path == "" || root == "" || !hasMarker(root) {
		t.Skipf("COURSE_TRAINING_STATION and %s (a niagara-research checkout) are not both set", RootEnv)
	}
	c, problems := load(root)
	if c == nil {
		t.Fatalf("%v", problems)
	}
	preds, st, err := verifyPredictions(c, path, os.Getenv("COURSE_NIAGARA_INSTALL"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.close()
	counts := map[string]int{}
	for _, p := range preds {
		counts[p.Verdict]++
		t.Logf("%s %s %s", p.Item.ID, p.Verdict, p.Reason)
		if p.Verdict == predContradicted {
			t.Errorf("%s CONTRADICTED: %s", p.Item.ID, p.Reason)
		}
		if !p.Item.hasStatus("predicted") {
			t.Errorf("%s is not a predicted item", p.Item.ID)
		}
	}
	if counts[predConfirmed] != 0 || counts[predPartial] == 0 {
		t.Errorf("want no CONFIRMED and some PARTIAL on the live station: %v", counts)
	}
}
