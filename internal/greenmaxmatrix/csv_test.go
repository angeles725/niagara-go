package greenmaxmatrix

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/angeles725/niagara-go/internal/cli"
)

// TestCSVMatchesPython compares the matrix CSV with the file Python wrote for the same inputs
// (testdata/csv, frozen with the golden cases).
func TestCSVMatchesPython(t *testing.T) {
	t.Chdir("testdata")
	for name, args := range map[string][]string{
		"matrix":          {"inputs/sup.bog", "inputs/jace.bog", "inputs/px"},
		"matrix-sup-only": {"inputs/sup.bog", "inputs/empty-a.bog", "inputs/px"},
		"matrix-px-empty": {"inputs/sup.bog", "inputs/jace.bog", "inputs/px-empty"},
	} {
		out := filepath.Join(t.TempDir(), "out.csv")
		var stdout, stderr bytes.Buffer
		if code := Main(append(args, "-o", out), cli.Stdio{Out: &stdout, Err: &stderr}); code != 0 {
			t.Fatalf("%s: exit %d: %s", name, code, stderr.String())
		}
		got, err := os.ReadFile(out)
		if err != nil {
			t.Fatal(err)
		}
		want, err := os.ReadFile(filepath.Join("csv", name+".csv"))
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(got, want) {
			t.Errorf("%s: CSV differs\n got: %q\nwant: %q", name, got, want)
		}
	}
}

func TestCSVRowQuoting(t *testing.T) {
	got := csvRow([]string{"a", `b "c"`, "d,e", "", " f", "g\nh"})
	if want := "a,\"b \"\"c\"\"\",\"d,e\",, f,\"g\nh\"\r\n"; got != want {
		t.Errorf("csvRow = %q, want %q", got, want)
	}
}
