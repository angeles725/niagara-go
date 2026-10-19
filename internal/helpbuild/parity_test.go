package helpbuild

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/angeles725/niagara-go/internal/pyref"
)

// The Python-vs-Go parity harness: testdata/parity/driver.py runs the
// niagara-help functions of the pinned Python reference (internal/pyref: the
// pre-go-retire worktree named by NIAGARA_HELP_PY_REF, decision D47) and
// prints JSON lines; the tests below rebuild every line from this package and
// require an empty diff. It needs python3 (CPython 3.15) and that worktree:
// plain `go test` skips visibly without them, a run that set a
// HELPBUILD_PARITY_* variable or PYREF_REQUIRED=1 (make parity) fails
// (missingReference). The live niagara-help checkout is only DATA
// (niagaraHelpData: versions/, the course); its Python is never read.

// parityDrivers are the Python sides of the parity tests.
var parityDrivers = []string{"textdriver.py", "sweepdriver.py"}

// driver is the parity interpreter and the tools/ folder of the niagara-help
// Python reference; the interpreter is checked first, so a wrong python3 is
// reported even without the reference.
func driver(t *testing.T) (python, tools string) {
	t.Helper()
	python, err := exec.LookPath("python3")
	if err != nil {
		missingReference(t, "python3 not on PATH: parity not checked")
	}
	for _, name := range parityDrivers {
		if _, err := os.Stat(filepath.Join("testdata", "parity", name)); err != nil {
			t.Fatalf("parity driver missing from the checkout: %v", err)
		}
	}
	v, err := pythonVersion(python)
	if err == nil {
		err = checkPythonMinor(v)
	}
	if err != nil {
		missingReference(t, fmt.Sprintf("parity driver interpreter %s: %v", python, err))
	}
	tools = filepath.Join(helpPyRef(t), "tools")
	if _, err := os.Stat(filepath.Join(tools, "niagara_help_lib", "jar_listing.py")); err != nil {
		t.Fatalf("the niagara-help Python reference has no tools/niagara_help_lib (%v): it is not the %s tree", err, pyref.Tag)
	}
	pythonLogged.Do(func() { t.Logf("parity driver python: CPython %s (%s), reference %s", v, python, filepath.Dir(tools)) })
	return python, tools
}

// parityPythonMinor is the CPython release the drivers' reference
// behaviour (html.parser above all) was pinned against.
const parityPythonMinor = "3.15"

var (
	pythonVersions sync.Map // interpreter path -> version string
	pythonLogged   sync.Once
)

// pythonVersion is the X.Y.Z of the interpreter at python, run once per
// path with the flags the drivers use.
func pythonVersion(python string) (string, error) {
	if v, ok := pythonVersions.Load(python); ok {
		return v.(string), nil
	}
	out, err := exec.Command(python, "-I", "-B", "-c",
		"import sys; print('%d.%d.%d' % sys.version_info[:3])").Output()
	if err != nil {
		return "", fmt.Errorf("cannot run it to read its version: %v", err)
	}
	v := strings.TrimSpace(string(out))
	pythonVersions.Store(python, v)
	return v, nil
}

// checkPythonMinor accepts a CPython parityPythonMinor.x version only.
func checkPythonMinor(v string) error {
	rest, ok := strings.CutPrefix(v, parityPythonMinor+".")
	if ok && rest != "" && strings.Trim(rest, "0123456789") == "" {
		return nil
	}
	return fmt.Errorf("it is CPython %q, the parity reference is CPython %s.x (html.parser and the other "+
		"stdlib behaviour the drivers compare change between minor releases): put a %s python3 first on PATH",
		v, parityPythonMinor, parityPythonMinor)
}
