package golden

import (
	"bytes"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/angeles725/niagara-go/internal/cli"
)

// Go-only cases (D34) hold the reviewed output of the Go port for behaviour its Python tool never
// had, so unlike the Python cases they are regenerated from the port itself.

// GoTestdata is a package's Go-only case set, relative to the package directory: cases under
// GoTestdata/golden, their synthetic inputs under GoTestdata/inputs.
const GoTestdata = "testdata/go"

// UpdateEnv names the variable that rewrites the Go-only expectations (any non-empty value;
// GOLDEN_UPDATE=1 by convention, the knob of the Python golden suites of the source repos).
const UpdateEnv = "GOLDEN_UPDATE"

// Updating reports whether UpdateEnv is set. A package's generator of synthetic Go-only inputs
// uses it to rewrite them under the same knob.
func Updating() bool { return os.Getenv(UpdateEnv) != "" }

// RunGo checks the Go-only cases under testdata/go/golden against m, as Run does. With UpdateEnv
// set it first rewrites each case's stdout, stderr and exit from m in process (a new case needs
// only its cmd.json; an empty stderr removes the file), then checks them; each rewritten file is
// logged, and the rewrite is reviewed in the diff before it is committed. GOLDEN_BIN affects only that check.
func RunGo(t *testing.T, m cli.Main) {
	t.Helper()
	runGo(t, GoTestdata, m)
}

func runGo(t *testing.T, testdata string, m cli.Main) {
	t.Helper()
	if Updating() {
		t.Logf("golden: %s set, Go-only cases under %s regenerated", UpdateEnv, testdata)
		update(t, testdata, m)
	}
	Run(t, testdata, m)
}

// update rewrites the outputs of every case directory under testdata/golden. Each case runs in
// its own subtest, so its working directory and env are restored before the next step.
func update(t *testing.T, testdata string, m cli.Main) {
	t.Helper()
	testdata, err := filepath.Abs(testdata)
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(testdata, "golden")
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	t.Run("update", func(t *testing.T) {
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			dir := filepath.Join(root, e.Name())
			c, err := loadCmd(dir)
			if err != nil {
				t.Fatalf("golden case %s: %v", e.Name(), err)
			}
			t.Run(e.Name(), func(t *testing.T) {
				skipUnlessHost(t, c)
				bound, tmp := bind(t, c)
				writeOutputs(t, dir, unbind(inProcess(t, testdata, m, bound), tmp))
			})
		}
	})
}

// writeOutputs rewrites a case's expectations and logs each file whose bytes changed, so a
// GOLDEN_UPDATE left exported shows in `go test -v`.
func writeOutputs(t *testing.T, dir string, r result) {
	t.Helper()
	rewritten, err := rewriteOutputs(dir, r)
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range rewritten {
		t.Logf("golden: rewrote %s", path)
	}
}

// rewriteOutputs writes stdout, stderr and exit of r under dir and returns the paths whose
// content changed (a created, modified or removed file).
func rewriteOutputs(dir string, r result) ([]string, error) {
	var changed []string
	put := func(name string, body []byte) error {
		path := filepath.Join(dir, name)
		old, err := os.ReadFile(path)
		if err == nil && bytes.Equal(old, body) {
			return nil
		}
		if err != nil && !os.IsNotExist(err) {
			return err
		}
		changed = append(changed, path)
		return os.WriteFile(path, body, 0o644)
	}
	if len(r.stderr) == 0 { // a missing stderr is an empty expectation
		path := filepath.Join(dir, "stderr")
		switch err := os.Remove(path); {
		case err == nil:
			changed = append(changed, path)
		case !os.IsNotExist(err):
			return nil, err
		}
	} else if err := put("stderr", r.stderr); err != nil {
		return nil, err
	}
	if err := put("stdout", r.stdout); err != nil {
		return nil, err
	}
	if err := put("exit", []byte(strconv.Itoa(r.exit)+"\n")); err != nil {
		return nil, err
	}
	return changed, nil
}
