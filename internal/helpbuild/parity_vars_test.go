package helpbuild

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/angeles725/niagara-go/internal/pyref"
)

// realParityVar is the umbrella switch of every test that reads a real
// install or the real corpus (minutes of work, so off by default, -short or
// not); the finer HELPBUILD_PARITY_* variables of a test also turn it on.
const realParityVar = "HELPBUILD_PARITY_REAL"

// paritySwitches are the real-data parity variables that are switches; the
// other HELPBUILD_PARITY_* variables hold a path or a path list.
var paritySwitches = map[string]bool{realParityVar: true, "HELPBUILD_PARITY_FULL": true, "HELPBUILD_PARITY_415": true}

// parityVar reports whether the real-data parity variable name is on: a
// switch when set to "1", a value variable when not empty. A switch set to
// anything else is an error naming the variable and the accepted value.
func parityVar(name string) (bool, error) {
	v := os.Getenv(name)
	if !paritySwitches[name] {
		return v != "", nil
	}
	switch v {
	case "":
		return false, nil
	case "1":
		return true, nil
	}
	return false, fmt.Errorf("%s=%q: the accepted value is 1 (unset turns it off)", name, v)
}

// parityOn is parityVar failing t on a malformed switch.
func parityOn(t *testing.T, name string) bool {
	t.Helper()
	on, err := parityVar(name)
	if err != nil {
		t.Fatal(err)
	}
	return on
}

// parityVars are every HELPBUILD_PARITY_* variable, switches first.
var parityVars = []string{realParityVar, "HELPBUILD_PARITY_FULL", "HELPBUILD_PARITY_415",
	"HELPBUILD_PARITY_JARS", "HELPBUILD_PARITY_PDF_DOCS", "HELPBUILD_PARITY_COURSE", "HELPBUILD_PARITY_BAJADOC",
	"HELPBUILD_PARITY_BAJADOCXML", "HELPBUILD_PARITY_INDEXES", "HELPBUILD_PARITY_COVERAGE", "HELPBUILD_PARITY_SWEEP",
	rebuildParityVar}

// parityRequested names the first HELPBUILD_PARITY_* variable set (as
// "NAME=value"), "" when none is: the run asked for parity, so a missing
// Python reference is a failure, not a skip.
func parityRequested() string {
	for _, v := range parityVars {
		if val := os.Getenv(v); val != "" {
			return v + "=" + val
		}
	}
	return ""
}

// missingReference skips t with msg in a plain run and fails it when the
// run requested parity (make parity).
func missingReference(t *testing.T, msg string) {
	t.Helper()
	if req := parityRequested(); req != "" {
		t.Fatalf("%s\n%s is set, so the Python reference is required: python3 (CPython 3.15) on PATH and the niagara-help "+
			"pre-go-retire worktree (%s, default %s); NIAGARA_HELP_DIR names only the data checkout", msg, req,
			helpPyRefEnv, pyref.DefaultDir("niagara-help"))
	}
	t.Skip(msg)
}

// A switch counts only when set to "1"; any other value fails naming the
// variable and the accepted value, so a typo never turns a test into a
// vacuous pass. A value variable (a path or a list) counts when non-empty.
func TestRealParityVars(t *testing.T) {
	for _, c := range []struct {
		name, value string
		on          bool
		err         string
	}{
		{realParityVar, "", false, ""},
		{realParityVar, "1", true, ""},
		{realParityVar, "true", false, realParityVar + `="true": the accepted value is 1`},
		{"HELPBUILD_PARITY_415", "0", false, `HELPBUILD_PARITY_415="0": the accepted value is 1`},
		{"HELPBUILD_PARITY_FULL", "yes", false, `HELPBUILD_PARITY_FULL="yes": the accepted value is 1`},
		{"HELPBUILD_PARITY_JARS", "a.jar", true, ""},
		{"HELPBUILD_PARITY_COURSE", "", false, ""},
	} {
		t.Setenv(c.name, c.value)
		on, err := parityVar(c.name)
		if on != c.on || (err == nil) != (c.err == "") || err != nil && !strings.Contains(err.Error(), c.err) {
			t.Errorf("%s=%q: on %v, err %v; want on %v, err %q", c.name, c.value, on, err, c.on, c.err)
		}
	}
}

// A run that set any HELPBUILD_PARITY_* variable requires the Python
// reference; a plain run does not.
func TestParityRequested(t *testing.T) {
	for _, v := range parityVars {
		t.Setenv(v, "")
	}
	if got := parityRequested(); got != "" {
		t.Errorf("no variable set: %q", got)
	}
	t.Setenv("HELPBUILD_PARITY_COURSE", "/c")
	t.Setenv("HELPBUILD_PARITY_FULL", "1")
	if got := parityRequested(); got != "HELPBUILD_PARITY_FULL=1" {
		t.Errorf("FULL and COURSE set: %q", got)
	}
}

// html.parser and the other stdlib behaviour the drivers pin change between
// CPython minor releases: only a 3.15.x interpreter is a valid reference.
func TestParityPythonMinor(t *testing.T) {
	for _, c := range []struct {
		version string
		ok      bool
	}{
		{"3.15.0", true}, {"3.15.12", true}, {"3.14.3", false}, {"3.16.0", false},
		{"3.1.5", false}, {"3.150.0", false}, {"", false}, {"Python 3.15.0", false},
	} {
		if err := checkPythonMinor(c.version); (err == nil) != c.ok {
			t.Errorf("%q: %v", c.version, err)
		}
	}
}

// pythonVersionQuery is the command line pythonVersion runs.
const pythonVersionQuery = "import sys; print('%d.%d.%d' % sys.version_info[:3])"

// fakeWrongPython puts first on an otherwise empty PATH a python3 that
// answers pythonVersion's query with 3.14.9 and fails on any other command
// line, and points NIAGARA_HELP_PY_REF at a folder that does not exist:
// driver checks the interpreter before the reference, so it stops at the
// version check.
func fakeWrongPython(t *testing.T) string {
	t.Helper()
	t.Setenv(helpPyRefEnv, filepath.Join(t.TempDir(), "no-ref"))
	bin := t.TempDir()
	python := filepath.Join(bin, "python3")
	script := "#!/bin/sh\n" +
		`if [ "$#" -eq 4 ] && [ "$1" = -I ] && [ "$2" = -B ] && [ "$3" = -c ] && [ "$4" = "` + pythonVersionQuery + `" ]; then` + "\n" +
		"\techo 3.14.9\n\texit 0\nfi\n" +
		`echo "fake python3: unexpected command line: $*" >&2` + "\nexit 2\n"
	if err := os.WriteFile(python, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)
	return python
}

// A python3 that is not CPython 3.15 is a missing reference: a plain run
// skips once the version was read (before the reference is looked up), as
// it does without python3 or the Python reference. A HELPBUILD_PARITY_* variable set to ""
// counts as unset: the run is still a plain one.
func TestParityWrongPythonSkips(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the fake python3 is a shell script")
	}
	for _, v := range parityVars {
		t.Setenv(v, "")
		if _, set := os.LookupEnv(v); !set {
			t.Fatalf("%s not set to the empty string", v)
		}
	}
	python := fakeWrongPython(t)
	var skipped bool
	t.Run("driver", func(t *testing.T) {
		defer func() { skipped = t.Skipped() }()
		driver(t)
	})
	if v, ok := pythonVersions.Load(python); !skipped || !ok || v != "3.14.9" {
		t.Errorf("CPython 3.14.9 in a plain run: skipped %v, version read %v %v; want a skip after reading 3.14.9", skipped, v, ok)
	}
}

// wrongPythonChild marks the test binary that TestParityWrongPythonFails
// runs: it only calls driver, whose failure is the result under test.
const wrongPythonChild = "HELPBUILD_TEST_WRONG_PYTHON_CHILD"

// When the run requested parity (a HELPBUILD_PARITY_* variable set, as
// make parity does), the same wrong python3 fails the test instead of
// skipping it. driver's t.Fatalf would fail this test too, so it runs in a
// child test binary whose exit status and output are checked.
func TestParityWrongPythonFails(t *testing.T) {
	if os.Getenv(wrongPythonChild) == "1" {
		driver(t)
		return
	}
	if runtime.GOOS == "windows" {
		t.Skip("the fake python3 is a shell script")
	}
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range [][2]string{{realParityVar, "1"}, {"HELPBUILD_PARITY_BAJADOC", "/some/root"}} {
		t.Run(c[0], func(t *testing.T) {
			for _, v := range parityVars {
				t.Setenv(v, "")
			}
			t.Setenv(c[0], c[1])
			fakeWrongPython(t)
			cmd := exec.Command(exe, "-test.run=^TestParityWrongPythonFails$", "-test.v")
			cmd.Env = append(os.Environ(), wrongPythonChild+"=1")
			out, err := cmd.CombinedOutput()
			want := c[0] + "=" + c[1] + " is set, so the Python reference is required"
			if err == nil || !strings.Contains(string(out), "--- FAIL: TestParityWrongPythonFails") ||
				!strings.Contains(string(out), want) || !strings.Contains(string(out), `CPython "3.14.9"`) {
				t.Errorf("CPython 3.14.9 with %s=%s: %v\n%s\nwant a failure naming %q", c[0], c[1], err, out, want)
			}
		})
	}
}
