package helpbuild

import (
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
