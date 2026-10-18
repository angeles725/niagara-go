package searchdoor

import (
	"reflect"
	"strings"
	"testing"

	"github.com/angeles725/niagara-go/internal/golden"
)

// courseKeys is the hermetic answer key of the item-ID routing tests and goldens:
// L10.22, L10.23, RA1.R1, RA1.V9–V14, X3 and P2.
const courseKeys = "testdata/course-keys"

// The course-item goldens are Go only (Python nsearch has no course lead), so they live
// apart from testdata/golden, which convert_goldens.py rewrites from the Python goldens.
func TestGoldenCourseItem(t *testing.T) {
	useFakes(t)
	golden.Run(t, "testdata/courseitem", Main)
}

func TestCourseItemIDs(t *testing.T) {
	for query, want := range map[string][]string{
		"L10.23":                         {"L10.23"},
		"what does RA1.V12 require?":     {"RA1.V12"},
		"(see L10.23.)":                  {"L10.23"},
		"exam P2 and X3":                 {"P2", "X3"},
		"D1.8, T1.1; LB.7 LA.2 RA2.S":    {"D1.8", "T1.1", "LB.7", "LA.2", "RA2.S"},
		"L9.9–L9.13 and L9.9 again":      {"L9.9", "L9.13"},
		"RA1.R3a traps":                  {"RA1.R3a"},
		"P1 P2 P3 P4 P5 P6":              {"P1", "P2", "P3", "P4", "P5"},
		"l10.23 x3 p2 ra1.v12":           nil, // IDs are upper case in a query
		"P2P PT100 BX3 XP2 L10 D2 RA1":   nil,
		"docs 4.14.0.162 and L10.23.4":   nil,
		"MANAGE_EXECUTION shutdownNow":   nil,
		"RA1.v12 RA1.12 RAX.D1 LC.1 L.1": nil,
	} {
		if got := courseItemIDs(query); !reflect.DeepEqual(got, want) {
			t.Errorf("courseItemIDs(%q) = %q, want %q", query, got, want)
		}
	}
}

// No question of the search golden set names an item ID: routing must not change them.
func TestGoldenQuestionsDoNotRoute(t *testing.T) {
	for _, row := range loadQualityRows(t) {
		if ids := courseItemIDs(row.Question); ids != nil {
			t.Errorf("%s %q routes to %q", row.ID, row.Question, ids)
		}
	}
}

func searchKeys(t *testing.T, query, keys string) []*Outcome {
	t.Helper()
	d, _, _ := newFakeDoor(nil)
	p := Paths{Blocks: "corpus-nav.py", Docs: "niagara_help.py", Code: "module_nav.py", Keys: keys}
	return d.search(query, p, defaultLimit, "", 1, "")
}

func itemKeys(o *Outcome) []string {
	var keys []string
	for _, it := range o.Items {
		keys = append(keys, it.Key)
	}
	return keys
}

func TestCourseItemLeadsTheSources(t *testing.T) {
	for query, want := range map[string][]string{
		"L10.23":                          {"L10.23"},
		"what does RA1.V12 require":       {"RA1.V9–V14"},
		"RA1.V10 RA1.V12 are one item":    {"RA1.V9–V14"},
		"exam P2 and X3":                  {"P2", "X3"},
		"L99.1 is not an item, L10.22 is": {"L10.22"},
	} {
		out := searchKeys(t, query, courseKeys)
		if len(out) != 4 || out[0].Key != "course" || out[0].State != OK {
			t.Errorf("%q: outcomes %d, first %+v; want the course item first, then the three sources", query, len(out), out[0])
			continue
		}
		if got := itemKeys(out[0]); !reflect.DeepEqual(got, want) {
			t.Errorf("%q: course items %q, want %q", query, got, want)
		}
		if keys := []string{out[1].Key, out[2].Key, out[3].Key}; !reflect.DeepEqual(keys, []string{"blocks", "docs", "code"}) {
			t.Errorf("%q: sources %q", query, keys)
		}
	}
}

func TestCourseItemBodyIsTheShowOutput(t *testing.T) {
	o := searchKeys(t, "L10.23", courseKeys)[0]
	it := o.Items[0]
	if it.Display != "L10.23 (lab item)" || o.Command != "course show L10.23" {
		t.Fatalf("display %q, command %q", it.Display, o.Command)
	}
	body := strings.Join(it.Body, "\n")
	for _, want := range []string{"  source:   docs/n4-dev-course-solutions/labs.md:", "  answer:", "ten CSV files"} {
		if !strings.Contains(body, want) {
			t.Errorf("body lacks %q:\n%s", want, body)
		}
	}
}

func TestNoItemNoRoute(t *testing.T) {
	for _, query := range []string{"alarm history", "L99.1 and P9 are not items", "P2P alarm"} {
		if out := searchKeys(t, query, courseKeys); len(out) != 3 {
			t.Errorf("%q: %d outcomes, want the three sources only", query, len(out))
		}
	}
}

func TestMissingAnswerKeyIsUnavailable(t *testing.T) {
	out := searchKeys(t, "L10.23", "testdata/no-such-checkout")
	if len(out) != 4 || out[0].State != Unavailable ||
		!strings.Contains(out[0].Reason, "has no docs/n4-dev-course-solutions/labs.md") {
		t.Fatalf("first outcome %+v", out[0])
	}
}
