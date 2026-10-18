package helpbuild

import (
	"errors"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var updateCov = flag.Bool("update-cov", false, "rewrite testdata/cov/golden/ from the Python reports (TestParity*Golden)")

// ModuleClasses matches without case and dashes, a trailing rt/ux dropped,
// as a prefix of the class's module, and sorts the classes.
func TestModuleClasses(t *testing.T) {
	base := covReportBase(t, t.TempDir())
	src, err := loadReportIndex(base, "source-index.json", true)
	if err != nil {
		t.Fatal(err)
	}
	for module, want := range map[string]string{
		"alarm-rt": "BAlarm BAlarmUx BAlarmWb", "ALARM-UX": "BAlarm BAlarmUx BAlarmWb", "alarm-wb": "BAlarmWb",
		"zeta": "BZeta Bx.txt", "École": "BÉcole", "nothing": "",
	} {
		got, err := ModuleClasses(src, module)
		if err != nil || strings.Join(got, " ") != want {
			t.Errorf("%s: %q %v; want %q", module, got, err, want)
		}
	}
}

// covGoldenCase is one report of a synthetic fixture checked in under
// testdata/cov/golden/<name>.jsonl: the driver's two lines, Python's
// output, the fixture paths shown by their labels.
type covGoldenCase struct {
	name, mode string
	args       []string
}

func covGoldenPath(name string) string {
	return filepath.Join("testdata", "cov", "golden", name+".jsonl")
}

// covRelabel joins lines and shows each path of pairs (path, label, ...)
// as its label; the longer paths come first.
func covRelabel(lines []string, pairs ...string) string {
	return strings.NewReplacer(pairs...).Replace(strings.Join(lines, "\n") + "\n")
}

// checkCovGoldens runs every case on the Go side (python false) or the
// Python side, and compares with the golden files; the Python side
// rewrites them with -update-cov.
func checkCovGoldens(t *testing.T, cases []covGoldenCase, python bool, pairs ...string) {
	t.Helper()
	for _, c := range cases {
		var got string
		if python {
			got = covRelabel(runCovDriver(t, append([]string{c.mode}, c.args...)...), pairs...)
		} else {
			got = covRelabel(goCov(c.mode, c.args...), pairs...)
		}
		if python && *updateCov {
			writeFile(t, covGoldenPath(c.name), []byte(got))
			continue
		}
		want, err := os.ReadFile(covGoldenPath(c.name))
		switch {
		case err != nil:
			t.Errorf("%s: %v", c.name, err)
		case string(want) != got && python:
			t.Errorf("%s: the golden file is not Python's output; rewrite it with -update-cov\n python %s\n golden %s", c.name, got, want)
		case string(want) != got:
			t.Errorf("%s:\n got %s\nwant %s", c.name, got, want)
		}
	}
}

// reportGoldenCases are the coverage report goldens: dir holds the report
// data root and no indexes of its own.
func reportGoldenCases(t *testing.T) (cases []covGoldenCase, pairs []string) {
	t.Helper()
	dir := t.TempDir()
	report := covReportBase(t, dir)
	return []covGoldenCase{
		{"coverage", "coverage", []string{report}},
		{"coverage-alarm", "coverage", []string{report, "alarm-rt"}},
		{"coverage-miss", "coverage", []string{report, "nothing"}},
		{"guides", "guides", []string{report}},
		{"missing-index", "coverage", []string{dir}},
	}, []string{report, "<report>", dir, "<dir>"}
}

// TestCoverageReportGolden: no Python needed; the Go reports of the
// synthetic fixture against their golden files.
func TestCoverageReportGolden(t *testing.T) {
	cases, pairs := reportGoldenCases(t)
	checkCovGoldens(t, cases, false, pairs...)
}

// TestParityCoverageReportGolden: the golden files are Python's output.
func TestParityCoverageReportGolden(t *testing.T) {
	covPython(t)
	cases, pairs := reportGoldenCases(t)
	checkCovGoldens(t, cases, true, pairs...)
}

// covMalformed are report indexes of an unexpected shape: the file
// replaced in the report fixture, its text, the module ("" = overview);
// python is what Python does (ok: a report and its status, partial: it
// raises after printing the lines before the failing lookup, empty: it
// raises before any line); shape is true when Go returns an
// *IndexShapeError before any output, false when its report is Python's.
var covMalformed = []struct {
	name, file, text, module, python string
	shape                            bool
}{
	{"class record not an object", "class-index.json", `{"BAlarmUx": [1]}`, "alarm-rt", "partial", true},
	{"class record not an object, overview", "class-index.json", `{"BAlarmUx": [1]}`, "", "ok", false},
	{"class index not an object, overview", "class-index.json", `["BAlarmUx"]`, "", "ok", true},
	{"method entries a number", "method-index.json", `{"ack": 5}`, "alarm-rt", "partial", true},
	{"method entries a non-empty string", "method-index.json", `{"ack": "x"}`, "alarm-rt", "partial", true},
	{"method entries a non-empty object", "method-index.json", `{"ack": {"class": "BAlarm"}}`, "alarm-rt", "partial", true},
	{"method entry not an object", "method-index.json", `{"ack": [1]}`, "alarm-rt", "partial", true},
	{"method entries a number, overview", "method-index.json", `{"ack": 5}`, "", "ok", false},
	{"method entries empty string and object", "method-index.json",
		`{"ack": [{"class": "BAlarm", "section": "methods"}], "e1": "", "e2": {}}`, "alarm-rt", "ok", false},
	{"module not a string", "source-index.json", `{"BAlarm": {"module": "alarm-rt"}, "BZeta": {"module": 5}}`, "alarm-rt", "empty", true},
	{"module not a string, overview tie", "source-index.json", `{"BAlarm": {"module": "alarm-rt"}, "BZeta": {"module": 5}}`, "", "partial", true},
	{"module not a string, overview", "source-index.json", `{"BAlarm": {"module": "alarm-rt"}, "BOrphan": {"module": 5}}`, "", "ok", true},
	{"source record not an object", "source-index.json", `{"BAlarm": {"module": "alarm-rt"}, "BZeta": 5}`, "alarm-rt", "empty", true},
	{"source record not an object, overview", "source-index.json", `{"BAlarm": {"module": "alarm-rt"}, "BZeta": 5}`, "", "partial", true},
	{"source index not an object", "source-index.json", `["BAlarm"]`, "alarm-rt", "empty", true},
	{"source index not an object, overview", "source-index.json", `["BAlarm"]`, "", "partial", true},
}

// TestCoverageMalformedIndexes: no Python needed; an index of an
// unexpected shape is an *IndexShapeError before any output, and empty
// string or object method entries count no method (Python iterates
// nothing in them).
func TestCoverageMalformedIndexes(t *testing.T) {
	src := covReportBase(t, t.TempDir())
	for _, c := range covMalformed {
		t.Run(c.name, func(t *testing.T) {
			base := filepath.Join(t.TempDir(), "b")
			copyTree(t, src, base)
			writeFile(t, filepath.Join(base, "indexes", c.file), []byte(c.text))
			var b strings.Builder
			_, err := Coverage(&b, base, c.module)
			var shape *IndexShapeError
			switch {
			case c.shape && (!errors.As(err, &shape) || b.Len() != 0):
				t.Errorf("err %v after %q; want an *IndexShapeError before any output", err, b.String())
			case !c.shape && err != nil:
				t.Errorf("err %v; want a report", err)
			case c.name == "method entries empty string and object" && !strings.Contains(b.String(), "Total methods: 1\n"):
				t.Errorf("report %q; want Total methods: 1", b.String())
			}
		})
	}
}
