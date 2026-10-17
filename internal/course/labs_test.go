package course

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
	parseLabs(c, splitLines(raw))
	return c
}

func TestParseLabsFixture(t *testing.T) {
	c := parseLabsFile(t, "testdata/root/docs/n4-dev-course-solutions/labs.md")
	if len(c.errs) > 0 {
		t.Fatalf("errors: %v", c.errs)
	}
	var ids []string
	for _, it := range c.Items {
		ids = append(ids, it.ID)
	}
	want := []string{"L1.1", "L1.2", "L1.3", "L1.4", "L1.5", "L1.6", "L2.1", "L2.2–L2.4", "L2.5", "L2.6", "LB.1", "LA.1"}
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
	}
	for _, ck := range checks {
		it := c.lookup(ck.id)
		if it == nil {
			t.Errorf("%s: not found", ck.id)
			continue
		}
		if !reflect.DeepEqual(it.Status, ck.status) || it.LiveRun != ck.live || !reflect.DeepEqual(it.Ranges, ck.ranges) {
			t.Errorf("%s: status %v live %v ranges %v; want %v %v %v", ck.id, it.Status, it.LiveRun, it.Ranges,
				ck.status, ck.live, ck.ranges)
		}
	}
	if g := c.group("LB"); g == nil || g.Ranges[0] != (Range{21, 24}) || c.lookup("LB.1").Subsection != "Customising the manager" {
		t.Errorf("Bonus lab group or subsection wrong: %+v", g)
	}
}

func TestParseLabsErrors(t *testing.T) {
	c := parseLabsFile(t, "testdata/dup-root/docs/n4-dev-course-solutions/labs.md")
	var got []string
	for _, e := range c.sortedErrors() {
		got = append(got, e.msg)
	}
	want := []string{"malformed item ID L1.x", "item L2.1 is outside its group L1",
		"duplicate ID L1.1 (first at " + labsFile + ":5)", "live-run ID L1.9 is not an item"}
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
