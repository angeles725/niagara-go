package key

import (
	"os"
	"reflect"
	"strings"
	"testing"
)

func parseLabsFile(t *testing.T, path string) *Corpus {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	c := newCorpus()
	parseLabs(c, SplitLines(raw))
	return c
}

func TestParseLabsFixture(t *testing.T) {
	c := parseLabsFile(t, "../testdata/root/docs/n4-dev-course-solutions/labs.md")
	if len(c.errs) > 0 {
		t.Fatalf("errors: %v", c.errs)
	}
	var ids []string
	for _, it := range c.Items {
		ids = append(ids, it.ID)
	}
	want := []string{"L1.1", "L1.2", "L1.3", "L1.4", "L1.5", "L1.6", "L2.1", "L2.2–L2.4", "L2.5", "L2.6", "LB.1", "LA.1", "LA.2"}
	if !reflect.DeepEqual(ids, want) {
		t.Errorf("ids = %v, want %v", ids, want)
	}
	checks := []struct {
		id     string
		status []string
		live   bool
		ranges []Range
	}{
		{"L1.3", []string{"unverified", "identical"}, false, []Range{{4, 6}}},
		{"L1.4", []string{"predicted"}, true, []Range{{7, 7}}},
		{"L1.5", nil, false, []Range{{8, 9}, {11, 12}}},
		{"l2.3", nil, false, []Range{{15, 16}}},
		{"L2.5", []string{"unverified", "corrected", "identical"}, true, []Range{{17, 18}}},
		{"L2.6", []string{"predicted"}, true, []Range{{19, 20}}},
		{"LA.1", []string{"verified"}, false, []Range{{25, 26}}},
		{"LA.2", []string{"verified"}, false, []Range{{26, 26}}},
	}
	for _, ck := range checks {
		it := c.Lookup(ck.id)
		if it == nil {
			t.Errorf("%s: not found", ck.id)
			continue
		}
		if !reflect.DeepEqual(it.Status, ck.status) || it.LiveRun != ck.live || !reflect.DeepEqual(it.Ranges, ck.ranges) {
			t.Errorf("%s: status %v live %v ranges %v; want %v %v %v", ck.id, it.Status, it.LiveRun, it.Ranges,
				ck.status, ck.live, ck.ranges)
		}
	}
	for id, want := range map[string][]string{"LA.1": {"offline"}, "LA.2": {"live"}, "L1.3": nil} {
		if it := c.Lookup(id); it == nil || !reflect.DeepEqual(it.Verified, want) {
			var got []string
			if it != nil {
				got = it.Verified
			}
			t.Errorf("%s: verified evidence %v (found %v), want %v", id, got, it != nil, want)
		}
	}
	g, lb := c.Group("LB"), c.Lookup("LB.1")
	switch {
	case g == nil || len(g.Ranges) != 1 || g.Ranges[0] != (Range{21, 24}):
		t.Errorf("Bonus lab group wrong: %+v", g)
	case lb == nil || lb.Subsection != "Customising the manager":
		t.Errorf("LB.1 missing or subsection wrong: %+v", lb)
	}
}

func TestParseLabsErrors(t *testing.T) {
	c := parseLabsFile(t, "../testdata/dup-root/docs/n4-dev-course-solutions/labs.md")
	var got []string
	for _, e := range c.sortedErrors() {
		got = append(got, e.msg)
	}
	want := []string{"malformed item ID L1.x", "item L2.1 is outside its group L1",
		"duplicate ID L1.1 (first at " + LabsFile + ":5)", "live-run ID L1.9 is not an item",
		"live-run range L1.4–L1.2 is reversed or crosses groups"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("errors:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

func TestSplitStep(t *testing.T) {
	cases := []struct{ head, step, answer string }{
		{"Do it (`LI:1`) · Done.", "Do it (`LI:1`)", "Done."},
		{"Do it · *Why?* (`LI:2`) · Because.", "Do it · *Why?* (`LI:2`)", "Because."},
		{"Use `a · b` · Answer with `c · d`.", "Use `a · b`", "Answer with `c · d`."},
		{"*Why restart?* The registry.", "*Why restart?*", "The registry."},
		{"*Observe* the jar (`LI:7`).", "*Observe* the jar (`LI:7`).", ""},
		{"`gradlew jar` (`LI:10`).", "`gradlew jar` (`LI:10`).", ""},
	}
	for _, c := range cases {
		if s, a := splitStep(c.head); s != c.step || a != c.answer {
			t.Errorf("splitStep(%q) = %q, %q; want %q, %q", c.head, s, a, c.step, c.answer)
		}
	}
}

func TestExpandRange(t *testing.T) {
	cases := map[string][]string{
		"L2.2–L2.4":  {"L2.2", "L2.3", "L2.4"},
		"RA1.V9–V11": {"RA1.V9", "RA1.V10", "RA1.V11"},
		"RA1.M1-M2":  {"RA1.M1", "RA1.M2"},
		"L2.2–L3.4":  nil,
		"L2.4–L2.2":  nil,
		"RA1.M1–R2":  nil,
	}
	for id, want := range cases {
		got, ok := expandRange(id)
		if ok != (want != nil) || !reflect.DeepEqual(got, want) {
			t.Errorf("expandRange(%q) = %v, %v; want %v", id, got, ok, want)
		}
	}
}

// A verified marker is live or offline; both are the verified status, each counted in its own column.
func TestVerifiedEvidence(t *testing.T) {
	cases := []struct {
		text     string
		verified bool
		evidence []string
	}{
		{"*(verified live 2026-10-15)* two files", true, []string{"live"}},
		{"*(verified offline 2026-10-18)* the jar", true, []string{"offline"}},
		{"*(verified offline 2026-10-18)* then *(verified live 2026-10-19)*", true, []string{"live", "offline"}},
		{"*(verified online 2026-10-18)*", false, nil},
		{"*(verified offline 18-10-2026)*", false, nil},
		{"verified offline 2026-10-18", false, nil},
	}
	for _, c := range cases {
		st, ev := markers(c.text), evidence(c.text)
		if hasVerified := reflect.DeepEqual(st, []string{"verified"}); hasVerified != c.verified || !reflect.DeepEqual(ev, c.evidence) {
			t.Errorf("%q: status %v evidence %v; want verified %v evidence %v", c.text, st, ev, c.verified, c.evidence)
		}
	}
	var n Counts
	for _, ev := range [][]string{{"live"}, {"offline"}, {"live", "offline"}, nil} {
		n.Add(&Item{Status: []string{"verified"}, Verified: ev})
	}
	// Evidence without the verified status (an item built by hand) is not counted: live and
	// offline never exceed verified.
	n.Add(&Item{Status: []string{"predicted"}, Verified: []string{"live", "offline"}})
	if n.Verified != 4 || n.Live != 2 || n.Offline != 2 {
		t.Errorf("counts %+v, want verified 4, live 2, offline 2", n)
	}
	it := &Item{Status: []string{"identical", "verified"}, Verified: []string{"live", "offline"}}
	if got := it.StatusLabels(); !reflect.DeepEqual(got, []string{"identical", "verified (live, offline)"}) {
		t.Errorf("labels %v", got)
	}
}

// The exam's file-level status line: "verified live." and "verified offline" give the problems
// that evidence, a plain "verified" none, and an unverified line neither status nor evidence.
func TestParseExamFileStatus(t *testing.T) {
	cases := []struct {
		status   string
		want     []string
		evidence []string
	}{
		{"verified live.", []string{"verified"}, []string{"live"}},
		{"verified offline.", []string{"verified"}, []string{"offline"}},
		{"verified.", []string{"verified"}, nil},
		{"verified on the station", []string{"verified"}, nil},
		{"unverified", []string{"unverified"}, nil},
		{"draft", nil, nil},
	}
	for _, c := range cases {
		corpus := newCorpus()
		parseExam(corpus, []string{"# Exam", "", "> **Status: " + c.status + "** Built.", "", "## 1. Problem 1 — Limiter", "Body."})
		it := corpus.Lookup("P1")
		if it == nil {
			t.Fatalf("%q: no P1", c.status)
		}
		if !reflect.DeepEqual(it.Status, c.want) || !reflect.DeepEqual(it.Verified, c.evidence) {
			t.Errorf("%q: status %v evidence %v; want %v %v", c.status, it.Status, it.Verified, c.want, c.evidence)
		}
	}
}
