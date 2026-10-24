package golden

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// An intended.json reason explains a recorded difference in prose, and prose that names a count
// ("counts 9 indexes") silently goes stale when the recorded output is re-recorded (it did twice in
// round 10). TestIntendedReasonCounts makes that mechanical: every "N <plural noun>" in a reason must
// equal a count of the same noun in the case's recorded output (the Go override or the Python bytes),
// whenever that output states one as a header: "INDEXES (14)", "TEXT SOURCES (10 sources ...)", or a
// line labelled by the noun alone, "INDEXES: 14" / "    Indexes:  16". Anything else is not a claim
// about the output and is left alone: a singular ("exit 1", "8148 kB"), a verb ("22 registered"), or a
// noun the output only counts under another label ("16 files" against "Java files: 53,485").

// claimRe is "N nouns" in a reason: a number (thousands commas allowed) directly before a plural.
var claimRe = regexp.MustCompile(`\b(\d[\d,]*)\s+([A-Za-z][A-Za-z-]*s)\b`)

// stem is the singular a count is matched on: indexes -> index, sources -> source, commands -> command.
func stem(word string) string {
	w := strings.ToLower(word)
	for _, suf := range []string{"xes", "ses", "ches", "shes"} {
		if strings.HasSuffix(w, suf) {
			return strings.TrimSuffix(w, "es")
		}
	}
	return strings.TrimSuffix(w, "s")
}

// outputCounts is every header count the output gives for the noun stem, case-insensitively,
// commas dropped: "NOUNS (N" anywhere, or "NOUNS: N" on a line whose label is the noun alone.
func outputCounts(output, st string) map[string]bool {
	noun := regexp.QuoteMeta(st) + `(?:es|s)?`
	res := []*regexp.Regexp{
		regexp.MustCompile(`(?i)\b` + noun + `\s*\(\s*(\d[\d,]*)\b`),
		regexp.MustCompile(`(?im)^[ \t]*` + noun + `:[ \t]*(\d[\d,]*)\b`),
	}
	counts := map[string]bool{}
	for _, re := range res {
		for _, m := range re.FindAllStringSubmatch(output, -1) {
			counts[strings.ReplaceAll(m[1], ",", "")] = true
		}
	}
	return counts
}

// reasonDrift returns one message per "N noun" in reason that disagrees with every count of that
// noun in outputs; nouns the outputs never count are skipped.
func reasonDrift(reason string, outputs ...string) []string {
	all := strings.Join(outputs, "\n")
	var drift []string
	for _, m := range claimRe.FindAllStringSubmatch(reason, -1) {
		n, st := strings.ReplaceAll(m[1], ",", ""), stem(m[2])
		counts := outputCounts(all, st)
		if len(counts) > 0 && !counts[n] {
			got := make([]string, 0, len(counts))
			for c := range counts {
				got = append(got, c)
			}
			sort.Strings(got)
			drift = append(drift, `"`+m[0]+`" but the recorded output counts `+st+": "+strings.Join(got, ", "))
		}
	}
	return drift
}

// reasons walks a decoded intended.json and calls f for every object with a "reason", with that
// object's own stdout/stderr strings (an override) as its outputs.
func reasons(v any, f func(reason string, outputs []string)) {
	switch x := v.(type) {
	case map[string]any:
		if r, ok := x["reason"].(string); ok {
			var outs []string
			for _, k := range []string{"stdout", "stderr"} {
				if s, ok := x[k].(string); ok {
					outs = append(outs, s)
				}
			}
			f(r, outs)
		}
		for _, e := range x {
			reasons(e, f)
		}
	case []any:
		for _, e := range x {
			reasons(e, f)
		}
	}
}

// moduleRoot is the directory holding go.mod above the test's working directory.
func moduleRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("no go.mod above the test directory")
		}
		dir = parent
	}
}

func TestReasonDrift(t *testing.T) {
	// The stale text #207 fixed: the reason still said 12 while the recorded output had 14.
	stale := reasonDrift("call-index retired (I3-5): so stats counts 12 indexes and has no CALL INDEX block",
		"INDEXES (14):\n  class-index.json  6,133 classes\nINDEXES: 14\n")
	if len(stale) != 1 || !strings.Contains(stale[0], `"12 indexes"`) {
		t.Fatalf("stale count not reported: %q", stale)
	}
	for _, c := range []struct{ reason, out string }{
		{"counts 14 indexes", "INDEXES (14):"},
		{"TEXT SOURCES lists 10 sources", "TEXT SOURCES (10 sources of the full-text index, 105.4 MB total):"},
		{"so HELP NAVIGATOR counts 16 indexes", "    Indexes:          16\n"},
		{"Python printed 11 indexes, Go 9", "Python: INDEXES (11)\nGo: INDEXES (9)"},
		{"Python exited 0 after the error; Go exits 1", "error: no such slot\n"}, // never counted: not a claim
		{"the C stack used 8148 kB", "Stack overflow\n"},
	} {
		if d := reasonDrift(c.reason, c.out); len(d) != 0 {
			t.Errorf("reasonDrift(%q) = %q, want none", c.reason, d)
		}
	}
	if d := reasonDrift("COMMANDS: 21 available; the reason says 20 commands", "COMMANDS: 21 available"); len(d) != 1 {
		t.Errorf("a wrong command count was not reported: %q", d)
	}
}

// TestIntendedReasonCounts checks every intended.json in the module against its case's recorded
// output: the override in the file itself plus the case directory's stdout and stderr (Python's).
func TestIntendedReasonCounts(t *testing.T) {
	root := moduleRoot(t)
	checked := 0
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && (d.Name() == ".git" || d.Name() == "node_modules") {
			return filepath.SkipDir
		}
		if d.IsDir() || d.Name() != "intended.json" {
			return nil
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		var v any
		if err := json.Unmarshal(raw, &v); err != nil {
			t.Errorf("%s: %v", path, err)
			return nil
		}
		var recorded []string
		for _, name := range []string{"stdout", "stderr"} {
			if b, err := os.ReadFile(filepath.Join(filepath.Dir(path), name)); err == nil {
				recorded = append(recorded, string(b))
			}
		}
		rel, _ := filepath.Rel(root, path)
		reasons(v, func(reason string, outs []string) {
			checked++
			for _, d := range reasonDrift(reason, append(outs, recorded...)...) {
				t.Errorf("%s: reason says %s", rel, d)
			}
		})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if checked == 0 {
		t.Fatal("no intended.json reason found: the walk proves nothing")
	}
}
