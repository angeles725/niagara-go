package check

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/angeles725/niagara-go/internal/course/key"
)

// Predicted selects the items still marked *(predicted)* (the tests' keep; the commands build
// theirs from --status with HasStatus).
func Predicted(it *key.Item) bool { return it.HasStatus("predicted") }

// verifyOn decides the predicted items of c with every check run on one station.
func verifyOn(t *testing.T, c *key.Corpus, path, install string) []Prediction {
	t.Helper()
	ss, err := OpenStations(LinkedChecks(c, Predicted), func(string) string { return path }, install, Trees{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(ss.Close)
	return Verify(c, ss, Predicted)
}

func TestDecidePrediction(t *testing.T) {
	r := func(id, verdict, ev string, notes ...string) LinkedResult {
		return LinkedResult{Result: Result{Check: &Check{ID: id}, Verdict: verdict, Evidence: ev}, Notes: notes}
	}
	pass, pass2 := r("C1.1", Pass, "found 1"), r("C1.2", Pass, "x = 1 (stored)")
	part := r("C1.5", Pass, "y stored", "the console line")
	part2 := r("C1.6", Pass, "w stored", "the console line", "the log")
	fail := r("C1.3", Fail, "found 0, want at least 1")
	skip := r("C1.4", Skip, "cannot read modules/x.jar")
	cases := []struct {
		results         []LinkedResult
		verdict, reason string
	}{
		{nil, Undecided, "no check covers it"},
		{[]LinkedResult{pass, pass2}, Confirmed, "C1.1 PASS: found 1; C1.2 PASS: x = 1 (stored)"},
		{[]LinkedResult{pass, fail, skip}, Contradicted, "C1.3 FAIL: found 0, want at least 1"},
		{[]LinkedResult{pass, skip}, Undecided, "C1.4 SKIP: cannot read modules/x.jar"},
		{[]LinkedResult{skip}, Undecided, "C1.4 SKIP: cannot read modules/x.jar"},
		{[]LinkedResult{part, fail}, Contradicted, "C1.3 FAIL: found 0, want at least 1"},
		{[]LinkedResult{part, skip}, Undecided, "C1.4 SKIP: cannot read modules/x.jar"},
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
		"L1.1: *(verified offline 2026-10-17)* evidence: C1.1 found 1; C1.2 x = 1 (stored)" {
		t.Errorf("suggestion %q (checks of no station kind: offline)", got)
	}
}

// The marker is live only when every linked check reads a station; a jar, source-text or
// annotation check makes it offline.
func TestSuggestionEvidence(t *testing.T) {
	r := func(id, kind string) LinkedResult {
		return LinkedResult{Result: Result{Check: &Check{ID: id, Kind: kind}, Verdict: Pass, Evidence: "ok"}}
	}
	cases := []struct {
		results []LinkedResult
		want    string
	}{
		{[]LinkedResult{r("C1", kindSlot), r("C2", kindLink), r("C3", kindFileText), r("C4", kindTrend)}, "live"},
		{[]LinkedResult{r("C1", kindFiles), r("C2", kindAudit), r("C3", kindLog), r("C4", kindType)}, "live"},
		{[]LinkedResult{r("C1", kindJarText)}, "offline"},
		{[]LinkedResult{r("C1", kindSourceText)}, "offline"},
		{[]LinkedResult{r("C1", kindAnnotation)}, "offline"},
		{[]LinkedResult{r("C1", kindSlot), r("C2", kindJarSigned)}, "offline"},
	}
	for i, c := range cases {
		if got := Suggestion("L1.1", "2026-10-18", c.results); !strings.HasPrefix(got, "L1.1: *(verified "+c.want+" 2026-10-18)*") {
			t.Errorf("case %d: %q, want the %s marker", i, got, c.want)
		}
	}
}

// Every kind the spec uses has an evidence kind: a new kind cannot default to live silently.
func TestEvidenceOfEveryKind(t *testing.T) {
	for _, ck := range Spec {
		if e := evidenceOf[ck.Kind]; e != key.EvidenceLive && e != key.EvidenceOffline {
			t.Errorf("%s: kind %s has no evidence kind", ck.ID, ck.Kind)
		}
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
	preds := verifyOn(t, c, goodStation, fixtureInstall)
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

// confirmedLive are the predicted items the live TrainingStation confirms. L10.24 was the one (the
// file checks of its exported CSVs) until labs.md marked it verified live on 2026-10-17: it is no
// longer predicted, so none is.
var confirmedLive = map[string]bool{}

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
	preds := verifyOn(t, c, path, os.Getenv("COURSE_NIAGARA_INSTALL"))
	counts := map[string]int{}
	for _, p := range preds {
		counts[p.Verdict]++
		t.Logf("%s %s %s", p.Item.ID, p.Verdict, p.Reason)
		if (p.Verdict == Confirmed) != confirmedLive[p.Item.ID] {
			t.Errorf("%s %s, want CONFIRMED only for %v", p.Item.ID, p.Verdict, confirmedLive)
		}
		if p.Verdict == Contradicted {
			t.Errorf("%s CONTRADICTED: %s", p.Item.ID, p.Reason)
		}
		if !p.Item.HasStatus("predicted") {
			t.Errorf("%s is not a predicted item", p.Item.ID)
		}
	}
	if counts[Confirmed] != len(confirmedLive) || counts[Partial] == 0 {
		t.Errorf("want CONFIRMED only %v and some PARTIAL on the live station: %v", confirmedLive, counts)
	}
}

// A check SKIPped only because its optional tree (--install, --source, --labsrc, --solutions) was
// not given does not lower what the other checks decide; any other SKIP still does.
func TestDecideAbsentTree(t *testing.T) {
	r := func(id, kind, verdict, ev string, absent bool, notes ...string) LinkedResult {
		return LinkedResult{Result: Result{Check: &Check{ID: id, Kind: kind}, Verdict: verdict, Evidence: ev, Absent: absent}, Notes: notes}
	}
	pass := r("C1.1", kindType, Pass, "found 1", false)
	part := r("C1.5", kindSlot, Pass, "y stored", false, "the console line")
	absent := r("C1.12", kindFileText, Skip, "no --labsrc DIR: x not checked", true)
	unreadable := r("C1.9", kindFiles, Skip, "cannot read /x: not a directory", false)
	fail := r("C1.3", kindLink, Fail, "found 0, want at least 1", false)
	for _, c := range []struct {
		results         []LinkedResult
		verdict, reason string
	}{
		{[]LinkedResult{pass, absent}, Confirmed, "C1.1 PASS: found 1; not run: C1.12 SKIP: no --labsrc DIR: x not checked"},
		{[]LinkedResult{part, absent}, Partial, "C1.5 PASS: y stored; not observed: the console line; not run: C1.12 SKIP: no --labsrc DIR: x not checked"},
		{[]LinkedResult{absent}, Undecided, "C1.12 SKIP: no --labsrc DIR: x not checked"},
		{[]LinkedResult{pass, unreadable}, Undecided, "C1.9 SKIP: cannot read /x: not a directory"},
		{[]LinkedResult{absent, fail}, Contradicted, "C1.3 FAIL: found 0, want at least 1"},
	} {
		if v, why := Decide(c.results); v != c.verdict || why != c.reason {
			t.Errorf("Decide = %s %q, want %s %q", v, why, c.verdict, c.reason)
		}
	}
	// The marker's evidence is that of the checks that ran.
	if e := Evidence([]LinkedResult{pass, absent}); e != key.EvidenceLive {
		t.Errorf("evidence %s, want live", e)
	}
	if s := Suggestion("L1.1", "2026-10-18", []LinkedResult{pass, absent}); s != "L1.1: *(verified live 2026-10-18)* evidence: C1.1 found 1" {
		t.Errorf("suggestion %q", s)
	}
}

// Station.Run marks a SKIP Absent only when a tree the check reads was not given.
func TestRunMarksAbsent(t *testing.T) {
	st, err := Open(writeStation(t), "", false)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		ck     Check
		absent bool
	}{
		{Check{Kind: kindJar, Jar: "x"}, true},
		{Check{Kind: kindSourceText, Glob: "a", Match: ".", Op: opMin, N: 1}, true},
		{Check{Kind: kindAnnotation, Glob: "A.java", Fact: factSlots, Want: "x"}, true},
		{Check{Kind: kindAnnotation, Root: rootSolutions, Glob: "A.java", Fact: factSlots, Want: "x"}, true},
		{Check{Kind: kindFiles, Root: rootLabsrc, Glob: "a", Op: opMin, N: 1}, true},
		{Check{Kind: kindFileText, Glob: "shared/a", Line: 1, Match: "("}, false}, // a bad pattern
	} {
		if r := st.Run(&c.ck); r.Verdict != Skip || r.Absent != c.absent {
			t.Errorf("%s %s: %s %q absent %v, want SKIP absent %v", c.ck.Kind, c.ck.Root, r.Verdict, r.Evidence, r.Absent, c.absent)
		}
	}
	st.labsrc = filepath.Join(t.TempDir(), "nosuch") // given but unreadable: not absent
	if r := st.Run(&Check{Kind: kindFiles, Root: rootLabsrc, Glob: "a", Op: opMin, N: 1}); r.Verdict != Skip || r.Absent {
		t.Errorf("unreadable tree: %s %q absent %v", r.Verdict, r.Evidence, r.Absent)
	}
}
