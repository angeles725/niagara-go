package helpbuild

import (
	"errors"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/angeles725/niagara-go/internal/helpbuild/internal/testkit"
	"github.com/angeles725/niagara-go/internal/index"
)

var updateCov = flag.Bool("update-cov", false, "rewrite testdata/cov/golden/ from the Go reports (review the diff)")

// moduleClasses matches without case and dashes, a trailing rt/ux dropped,
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
		got, err := moduleClasses(src, module)
		if err != nil || strings.Join(got, " ") != want {
			t.Errorf("%s: %q %v; want %q", module, got, err, want)
		}
	}
}

// covGoldenCase is one report of a synthetic fixture checked in under
// testdata/cov/golden/<name>.jsonl: the two lines of goCov (Python's output,
// frozen when D47 retired the Python reference), the fixture paths shown by
// their labels.
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

// checkCovGoldens runs every case and compares it with its golden file;
// -update-cov rewrites the files from the Go reports.
func checkCovGoldens(t *testing.T, cases []covGoldenCase, pairs ...string) {
	t.Helper()
	for _, c := range cases {
		got := covRelabel(goCov(c.mode, c.args...), pairs...)
		if *updateCov {
			testkit.WriteFile(t, covGoldenPath(c.name), []byte(got))
			continue
		}
		want, err := os.ReadFile(covGoldenPath(c.name))
		switch {
		case err != nil:
			t.Errorf("%s: %v", c.name, err)
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

// TestCoverageReportGolden: the Go reports of the synthetic fixture
// against their golden files.
func TestCoverageReportGolden(t *testing.T) {
	cases, pairs := reportGoldenCases(t)
	checkCovGoldens(t, cases, pairs...)
}

// covMalformed are report indexes of an unexpected shape: the file
// replaced in the report fixture, its text, the module ("" = overview);
// python is what Python did (ok: a report and its status, partial: it
// raised after printing the lines before the failing lookup, empty: it
// raised before any line); shape is true when Go returns an
// *indexShapeError before any output, false when its report is Python's
// (TestCoverageMalformedFrozen).
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

// TestCoverageMalformedIndexes: an index of an
// unexpected shape is an *indexShapeError before any output, and empty
// string or object method entries count no method (Python iterates
// nothing in them).
func TestCoverageMalformedIndexes(t *testing.T) {
	src := covReportBase(t, t.TempDir())
	for _, c := range covMalformed {
		t.Run(c.name, func(t *testing.T) {
			base := filepath.Join(t.TempDir(), "b")
			testkit.CopyTree(t, src, base)
			testkit.WriteFile(t, filepath.Join(base, "indexes", c.file), []byte(c.text))
			var b strings.Builder
			_, err := Coverage(&b, base, c.module)
			var shape *indexShapeError
			switch {
			case c.shape && (!errors.As(err, &shape) || b.Len() != 0):
				t.Errorf("err %v after %q; want an *indexShapeError before any output", err, b.String())
			case !c.shape && err != nil:
				t.Errorf("err %v; want a report", err)
			case c.name == "method entries empty string and object" && !strings.Contains(b.String(), "Total methods: 1\n"):
				t.Errorf("report %q; want Total methods: 1", b.String())
			}
		})
	}
}

// TestGuideCoverageCountsThePagesItConverts: the guide coverage counts the pages the
// guides parser converts (.html, .htm, .md, a non-licence .txt) against their own
// guides-clean text, so the numerator can never exceed the denominator: before, a .md or
// .txt page's text was counted over an HTML-only denominator (4.15 printed 5940/5936).
func TestGuideCoverageCountsThePagesItConverts(t *testing.T) {
	base := t.TempDir()
	for _, f := range []string{"guides/G/a.html", "guides/G/b.htm", "guides/G/rc/c.md", "guides/G/notes.txt",
		"guides/G/license.txt", "guides/G/img.png", "guides/H/x.html", "guides/H/y.html",
		"guides-clean/G/a.txt", "guides-clean/G/b.htm.txt", "guides-clean/G/rc/c.md.txt", "guides-clean/G/notes.txt",
		"guides-clean/H/x.txt", "guides-clean/H/stale.txt"} {
		testkit.WriteFile(t, filepath.Join(base, filepath.FromSlash(f)), []byte("t"))
	}
	var out strings.Builder
	gaps, err := printGuideCoverage(&out, base)
	if err != nil {
		t.Fatal(err)
	}
	want := "GUIDE COVERAGE (guides/ pages vs their guides-clean/ text)\n" + strings.Repeat("=", 60) + "\n" +
		"Overall: 5/6 guide pages indexed (.html .htm .md .txt, licence texts excluded) across 2 folders, 1 gap folder(s)\n" +
		"  GAP      1/2     H\n\n"
	if gaps != 1 || out.String() != want {
		t.Errorf("gaps %d, report:\n%s\nwant:\n%s", gaps, out.String(), want)
	}
}

// TestOverviewNamesItsDenominator: the overview percentage is over the classes with
// indexed source (source-index.json), not over every class nh knows; the line says so.
func TestOverviewNamesItsDenominator(t *testing.T) {
	var out strings.Builder
	path := filepath.Join(t.TempDir(), "source-index.json")
	testkit.WriteFile(t, path, []byte(`{"BA": {"module": "m-rt"}, "BB": {"module": "m-rt"}}`))
	source, err := index.LoadObject(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := overviewCoverage(&out, source, &moduleGroups{}, map[string]bool{"BA": true}); err != nil {
		t.Fatal(err)
	}
	if want := "\nOverall: 1/2 classes documented (50.0% of the 2 classes with indexed source)\n"; !strings.Contains(out.String(), want) {
		t.Errorf("overview:\n%s\nwant line %q", out.String(), want)
	}
}
