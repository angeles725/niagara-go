package hdbread

import (
	"bytes"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/angeles725/niagara-go/internal/cli"
	"github.com/angeles725/niagara-go/internal/golden"
)

// Go-only golden cases (docs/goldens.md "Go-only cases"): behaviour added after the Python tool
// froze (D34), so stdout/stderr/exit hold the reviewed Go output, not a Python capture. Inputs
// are the synthetic stores of fixtures(), committed under testdata/go/inputs.
var update = flag.Bool("update", false, "rewrite testdata/go: the synthetic inputs and the Go-only case outputs")

const goTestdata = "testdata/go"

// TestGoInputs fails when a committed input differs from what fixtures() builds.
func TestGoInputs(t *testing.T) {
	for name, want := range fixtures() {
		path := filepath.Join(goTestdata, "inputs", name)
		if *update {
			if err := os.WriteFile(path, want, 0o644); err != nil {
				t.Fatal(err)
			}
			continue
		}
		got, err := os.ReadFile(path)
		if err != nil || !bytes.Equal(got, want) {
			t.Errorf("%s is not what fixtures() builds (err %v); run go test ./internal/hdbread -run TestGo -update", path, err)
		}
	}
}

// TestGoldenGo runs the Go-only cases; -update first rewrites each case's outputs from Main.
func TestGoldenGo(t *testing.T) {
	if *update {
		writeGoOutputs(t)
	}
	golden.Run(t, goTestdata, Main)
}

func writeGoOutputs(t *testing.T) {
	root, err := filepath.Abs(filepath.Join(goTestdata, "golden"))
	if err != nil {
		t.Fatal(err)
	}
	dirs, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	t.Chdir(goTestdata)
	for _, d := range dirs {
		dir := filepath.Join(root, d.Name())
		raw, err := os.ReadFile(filepath.Join(dir, "cmd.json"))
		if err != nil {
			t.Fatal(err)
		}
		var c struct {
			Args []string `json:"args"`
		}
		if err := json.Unmarshal(raw, &c); err != nil {
			t.Fatalf("%s: %v", dir, err)
		}
		var out, errb bytes.Buffer
		code := Main(c.Args, cli.Stdio{In: bytes.NewReader(nil), Out: &out, Err: &errb})
		files := map[string][]byte{"stdout": out.Bytes(), "stderr": errb.Bytes(),
			"exit": []byte(strconv.Itoa(code) + "\n")}
		for name, data := range files {
			path := filepath.Join(dir, name)
			if name == "stderr" && len(data) == 0 {
				os.Remove(path) // a missing stderr is an empty expectation
				continue
			}
			if err := os.WriteFile(path, data, 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}
}
