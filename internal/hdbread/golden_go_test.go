package hdbread

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/angeles725/niagara-go/internal/golden"
)

// Go-only golden cases (docs/goldens.md "Go-only cases"): behaviour added after the Python tool
// froze (D34), so stdout/stderr/exit hold the reviewed Go output, not a Python capture. Inputs
// are the synthetic stores of fixtures(), committed under testdata/go/inputs. GOLDEN_UPDATE=1
// rewrites both: the inputs here, the case outputs in golden.RunGo.

// TestGoInputs fails when a committed input differs from what fixtures() builds.
func TestGoInputs(t *testing.T) {
	for name, want := range fixtures() {
		path := filepath.Join(golden.GoTestdata, "inputs", name)
		if golden.Updating() {
			if err := os.WriteFile(path, want, 0o644); err != nil {
				t.Fatal(err)
			}
			continue
		}
		got, err := os.ReadFile(path)
		if err != nil || !bytes.Equal(got, want) {
			t.Errorf("%s is not what fixtures() builds (err %v); run GOLDEN_UPDATE=1 go test ./internal/hdbread -run TestGo", path, err)
		}
	}
}

// TestGoldenGo runs the Go-only cases (GOLDEN_UPDATE rewrites their outputs from Main first).
func TestGoldenGo(t *testing.T) { golden.RunGo(t, Main) }
