package check

import (
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/angeles725/niagara-go/internal/course/key"
)

func TestDecidePrediction(t *testing.T) {
	r := func(id, verdict, ev string, notes ...string) LinkedResult {
		return LinkedResult{Result: Result{Check: &Check{ID: id}, Verdict: verdict, Evidence: ev}, Notes: notes}
	}
	pass, pass2 := r("C1.1", Pass, "found 1"), r("C1.2", Pass, "x = 1 (stored)")
	part := r("C1.5", Pass, "y stored", "the console line")
	part2 := r("C1.6", Pass, "w stored", "the console line", "the log")
	fail := r("C1.3", Fail, "found 0, want at least 1")
	skip := r("C1.4", Skip, "no --install DIR")
	cases := []struct {
		results         []LinkedResult
		verdict, reason string
	}{
		{nil, Undecided, "no check covers it"},
		{[]LinkedResult{pass, pass2}, Confirmed, "C1.1 PASS: found 1; C1.2 PASS: x = 1 (stored)"},
		{[]LinkedResult{pass, fail, skip}, Contradicted, "C1.3 FAIL: found 0, want at least 1"},
		{[]LinkedResult{pass, skip}, Undecided, "C1.4 SKIP: no --install DIR"},
		{[]LinkedResult{skip}, Undecided, "C1.4 SKIP: no --install DIR"},
		{[]LinkedResult{part, fail}, Contradicted, "C1.3 FAIL: found 0, want at least 1"},
		{[]LinkedResult{part, skip}, Undecided, "C1.4 SKIP: no --install DIR"},
		// Every passing check observes only the stored part: PARTIAL, with what remains.
		{[]LinkedResult{part, part2}, Partial,
			"C1.5 PASS: y stored; C1.6 PASS: w stored; not observed: the console line; the log"},
		// One check that observes the whole answer is enough.
		{[]LinkedResult{part, pass}, Confirmed, "C1.5 PASS: y stored; C1.1 PASS: found 1"},
	}
	for _, c := range cases {
		if v, why := Decide(c.results); v != c.verdict || why != c.reason {
			t.Errorf("Decide(%d results) = %s %q, want %s %q", len(c.results), v, why, c.verdict, c.reason)
		}
	}
	if got := Suggestion("L1.1", "2026-10-17", []LinkedResult{pass, pass2}); got !=
		"L1.1: *(verified live 2026-10-17)* evidence: C1.1 found 1; C1.2 x = 1 (stored)" {
		t.Errorf("suggestion %q", got)
	}
}

// itemLinks reads a note under the key that matched (a covered ID of a range item), not under
// the item's ID, and links a check once however many of its keys reach the item.
func TestItemLinks(t *testing.T) {
	c, problems := key.Load("../testdata/root") // holds the range item L2.2–L2.4
	if c == nil {
		t.Fatalf("%v", problems)
	}
	spec := []Check{
		{ID: "C2.1", Keys: []string{"L2.3"}, Unobserved: map[string]string{"L2.3": "a view"}},
		{ID: "C2.2", Keys: []string{"L2.2", "L2.4"}, Unobserved: map[string]string{"L2.4": "a log"}},
		{ID: "C2.3", Keys: []string{"L2.2", "L2.9"}},
	}
	ls := itemLinks(c, spec)[c.Lookup("L2.3")]
	if len(ls) != 3 || ls[0].Check.ID != "C2.1" || !slices.Equal(ls[0].Notes, []string{"a view"}) ||
		ls[1].Check.ID != "C2.2" || !slices.Equal(ls[1].Notes, []string{"a log"}) || ls[2].Check.ID != "C2.3" || ls[2].Notes != nil {
		t.Errorf("links of L2.2–L2.4: %+v", ls)
	}
}

// A key reaches a range item through a covered ID: its note still applies (PARTIAL, no
// suggestion), and a check whose several keys reach one item is linked once.
func TestVerifyPredictionsThroughRange(t *testing.T) {
	c, problems := key.Load("../testdata/predict-root")
	if c == nil {
		t.Fatalf("%v", problems)
	}
	preds, st, err := VerifyPredictions(c, goodStation, fixtureInstall)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	got := map[string]Prediction{}
	for _, p := range preds {
		got[p.Item.ID] = p
	}
	if p := got["L4.15–L4.16"]; p.Verdict != Partial || !strings.Contains(p.Reason, "not observed: the outputs at 72.5") {
		t.Errorf("range L4.15–L4.16: %s %q, want PARTIAL with the C4.6 note", p.Verdict, p.Reason)
	}
	if p := got["L10.6–L10.7"]; p.Verdict != Confirmed || len(p.Results) != 1 {
		t.Errorf("range L10.6–L10.7: %s with %d results, want CONFIRMED from C10.7 once", p.Verdict, len(p.Results))
	}
}

// TestVerifyPredictionsTrainingStation runs verify-predictions on the live TrainingStation
// against the real answer key; it pins only what the live check test pins: C3.4 is the one
// FAIL, so no predicted item may be CONTRADICTED (L3.7, its key, is not predicted).
func TestVerifyPredictionsTrainingStation(t *testing.T) {
	path, root := os.Getenv("COURSE_TRAINING_STATION"), os.Getenv(key.RootEnv)
	if path == "" || root == "" || !key.HasMarker(root) {
		t.Skipf("COURSE_TRAINING_STATION and %s (a niagara-research checkout) are not both set", key.RootEnv)
	}
	c, problems := key.Load(root)
	if c == nil {
		t.Fatalf("%v", problems)
	}
	preds, st, err := VerifyPredictions(c, path, os.Getenv("COURSE_NIAGARA_INSTALL"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	counts := map[string]int{}
	for _, p := range preds {
		counts[p.Verdict]++
		t.Logf("%s %s %s", p.Item.ID, p.Verdict, p.Reason)
		if p.Verdict == Contradicted {
			t.Errorf("%s CONTRADICTED: %s", p.Item.ID, p.Reason)
		}
		if !p.Item.HasStatus("predicted") {
			t.Errorf("%s is not a predicted item", p.Item.ID)
		}
	}
	if counts[Confirmed] != 0 || counts[Partial] == 0 {
		t.Errorf("want no CONFIRMED and some PARTIAL on the live station: %v", counts)
	}
}
