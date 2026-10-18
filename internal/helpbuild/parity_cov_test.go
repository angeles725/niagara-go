package helpbuild

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/angeles725/niagara-go/internal/pyjson"
	"github.com/angeles725/niagara-go/internal/pyref"
)

// Parity of the coverage reports through testdata/cov/covdriver.py: the
// printed bytes and the return value (or the exception) of cmd_coverage and
// cmd_guide_coverage against Coverage and PrintGuideCoverage (and, in
// parity_covdocjars_test.go, cmd_doc_jar_coverage against DocJarCoverage).

// covPython is the Python side of the coverage parity: driver's CPython
// 3.15 python3 and the niagara-help tools of the pre-go-retire reference
// (internal/pyref); a missing reference skips, or fails under make parity
// (PYREF_REQUIRED=1, or a HELPBUILD_PARITY_* variable set).
func covPython(t *testing.T) (python, tools string) {
	t.Helper()
	return driver(t)
}

// runCovDriver runs covdriver.py and returns its two lines.
func runCovDriver(t *testing.T, args ...string) []string {
	t.Helper()
	python, tools := covPython(t)
	script, err := filepath.Abs(filepath.Join("testdata", "cov", "covdriver.py"))
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(python, append([]string{"-I", "-B", script, tools}, args...)...)
	cmd.Env = append(os.Environ(), pyref.Env("niagara-help")...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("covdriver %v: %v\n%s", args, err, stderr.String())
	}
	// Intended difference (D34): the MissingIndexError hint names the Go
	// command, nh rebuild-all, where Python names its own script.
	text := strings.ReplaceAll(string(out), pyRebuildHint, goRebuildHint)
	return strings.Split(strings.TrimSuffix(text, "\n"), "\n")
}

// The rebuild hint of MissingIndexError, as Python and as Go spell it.
const (
	pyRebuildHint = "with: python3 tools/niagara_help.py rebuild-all"
	goRebuildHint = "with: nh rebuild-all"
)

// covResult renders a Go run as the driver's lines.
func covResult(out string, status pyjson.Value, err error) []string {
	last := dumpLine(str("status"), status)
	var missing *MissingIndexError
	switch {
	case errors.As(err, &missing):
		last = dumpLine(str("missing"), str(err.Error()))
	case err != nil:
		last = dumpLine(str("raise"))
	}
	return []string{dumpLine(str("out"), str(out)), last}
}

// covGoModes are the Go sides of the driver modes other files add
// (docjars, parity_covdocjars_test.go).
var covGoModes = map[string]func(args ...string) []string{}

// goCov is the driver's mode rebuilt from this package.
func goCov(mode string, args ...string) []string {
	var b strings.Builder
	switch mode {
	case "coverage":
		module := ""
		if len(args) > 1 {
			module = args[1]
		}
		res, err := Coverage(&b, args[0], module)
		status := pyjson.Value{}
		switch {
		case res.Miss:
			b.WriteString(dumpLine(str("miss"), str("coverage"), str(module), str("modules")) + "\n")
		case module == "":
			status = intV(res.GuideGaps)
		}
		return covResult(b.String(), status, err)
	case "guides":
		n, err := PrintGuideCoverage(&b, args[0])
		return covResult(b.String(), intV(n), err)
	}
	if run, ok := covGoModes[mode]; ok {
		return run(args...)
	}
	panic("unknown mode " + mode)
}

// covBoth compares one mode on both sides; pyArgs and goArgs differ only in
// the data root (each side has its own copy), shown as <base>.
func covBoth(t *testing.T, what, mode string, pyArgs, goArgs []string, pyBase, goBase string) []string {
	t.Helper()
	py := runCovDriver(t, append([]string{mode}, pyArgs...)...)
	gol := goCov(mode, goArgs...)
	for i := range py {
		py[i] = strings.ReplaceAll(py[i], pyBase, "<base>")
	}
	for i := range gol {
		gol[i] = strings.ReplaceAll(gol[i], goBase, "<base>")
	}
	diffLines(t, what, py, gol)
	return py
}

// covCopies copies the data root src twice (Python's and Go's).
func covCopies(t *testing.T, src string) (py, gol string) {
	t.Helper()
	py, gol = filepath.Join(t.TempDir(), "py"), filepath.Join(t.TempDir(), "go")
	copyTree(t, src, py)
	copyTree(t, src, gol)
	return py, gol
}

// TestParityCoverageReport: the overview, module reports (a prefix match
// over rt/ux/wb, undocumented classes with and without a package, a module
// that matches nothing), the guide coverage alone, an empty and a missing
// index.
func TestParityCoverageReport(t *testing.T) {
	covPython(t)
	py, gol := covCopies(t, covReportBase(t, t.TempDir()))
	for _, c := range [][]string{
		{"coverage"}, {"coverage", "alarm-rt"}, {"coverage", "ALARM"}, {"coverage", "alarm-ux"}, {"coverage", "zeta-rt"},
		{"coverage", "école"}, {"coverage", "rt"}, {"coverage", "nothing"}, {"guides"},
	} {
		mode, rest := c[0], c[1:]
		t.Run(strings.Join(c, " "), func(t *testing.T) {
			covBoth(t, strings.Join(c, " "), mode, append([]string{py}, rest...), append([]string{gol}, rest...), py, gol)
		})
	}
	t.Run("empty source index", func(t *testing.T) {
		p, g := covCopies(t, py)
		for _, b := range []string{p, g} {
			writeFile(t, filepath.Join(b, "indexes", "source-index.json"), []byte(`{}`))
			if err := os.RemoveAll(filepath.Join(b, "guides")); err != nil {
				t.Fatal(err)
			}
		}
		covBoth(t, "empty", "coverage", []string{p}, []string{g}, p, g)
	})
	for _, name := range []string{"class-index.json", "method-index.json", "slots-index.json"} {
		t.Run("missing "+name, func(t *testing.T) {
			p, g := covCopies(t, py)
			for _, b := range []string{p, g} {
				if err := os.Remove(filepath.Join(b, "indexes", name)); err != nil {
					t.Fatal(err)
				}
			}
			covBoth(t, "missing "+name, "coverage", []string{p, "alarm"}, []string{g, "alarm"}, p, g)
		})
	}
}

// TestParityCoverageMalformed pins the intended difference on indexes of
// an unexpected shape (covMalformed): Go returns an *IndexShapeError
// before any output where Python raises after printing the lines before
// the failing lookup (partial) or before any line (empty), and where
// Python reads no bad value (ok) the Go report is Python's byte for byte,
// except the rows Go rejects up front.
func TestParityCoverageMalformed(t *testing.T) {
	covPython(t)
	src := covReportBase(t, t.TempDir())
	for _, c := range covMalformed {
		t.Run(c.name, func(t *testing.T) {
			p, g := covCopies(t, src)
			for _, b := range []string{p, g} {
				writeFile(t, filepath.Join(b, "indexes", c.file), []byte(c.text))
			}
			args := func(base string) []string {
				if c.module == "" {
					return []string{base}
				}
				return []string{base, c.module}
			}
			if c.python == "ok" && !c.shape {
				covBoth(t, c.name, "coverage", args(p), args(g), p, g)
				return
			}
			py := runCovDriver(t, append([]string{"coverage"}, args(p)...)...)
			if len(py) != 2 {
				t.Fatalf("covdriver gave %d lines, want the output line and the outcome: %q", len(py), py)
			}
			var out string
			if err := json.Unmarshal([]byte(strings.TrimSuffix(strings.TrimPrefix(py[0], `["out", `), "]")), &out); err != nil {
				t.Fatal(err)
			}
			raised := py[1] == `["raise"]`
			switch got := map[bool]string{true: "empty", false: "partial"}[out == ""]; {
			case c.python == "ok" && raised, c.python != "ok" && !raised:
				t.Errorf("python %s, want %s", py[1], c.python)
			case c.python != "ok" && got != c.python:
				t.Errorf("python raised after %q, want %s output", out, c.python)
			}
		})
	}
}
