package course

import (
	"os"
	"testing"
)

func TestDecidePrediction(t *testing.T) {
	pass := Result{Check: &Check{ID: "C1.1"}, Verdict: verdictPass, Evidence: "found 1"}
	pass2 := Result{Check: &Check{ID: "C1.2"}, Verdict: verdictPass, Evidence: "x = 1 (stored)"}
	fail := Result{Check: &Check{ID: "C1.3"}, Verdict: verdictFail, Evidence: "found 0, want at least 1"}
	skip := Result{Check: &Check{ID: "C1.4"}, Verdict: verdictSkip, Evidence: "no --install DIR"}
	cases := []struct {
		results         []Result
		verdict, reason string
	}{
		{nil, predUndecided, "no check covers it"},
		{[]Result{pass, pass2}, predConfirmed, "C1.1 PASS: found 1; C1.2 PASS: x = 1 (stored)"},
		{[]Result{pass, fail, skip}, predContradicted, "C1.3 FAIL: found 0, want at least 1"},
		{[]Result{pass, skip}, predUndecided, "C1.4 SKIP: no --install DIR"},
		{[]Result{skip}, predUndecided, "C1.4 SKIP: no --install DIR"},
	}
	for _, c := range cases {
		if v, r := decide(c.results); v != c.verdict || r != c.reason {
			t.Errorf("decide(%d results) = %s %q, want %s %q", len(c.results), v, r, c.verdict, c.reason)
		}
	}
	if got := suggestion("L1.1", "2026-10-17", []Result{pass, pass2}); got !=
		"L1.1: *(verified live 2026-10-17)* evidence: C1.1 found 1; C1.2 x = 1 (stored)" {
		t.Errorf("suggestion %q", got)
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
	if counts[predConfirmed] == 0 {
		t.Errorf("no predicted item confirmed on the live station: %v", counts)
	}
}
