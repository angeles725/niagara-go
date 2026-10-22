package greenmaxmatrix

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/angeles725/niagara-go/internal/cli"
)

func runMatrix(t *testing.T, args ...string) (code int, stdout, stderr string) {
	t.Helper()
	var out, errb bytes.Buffer
	code = Main(args, cli.Stdio{Out: &out, Err: &errb})
	return code, out.String(), errb.String()
}

// A px_dir without one GreenMAX Px file still writes the matrix (every Tablero "(no Px)", the CSV
// unchanged) but says so on stderr: a wrong folder must not pass silently.
func TestNoPxFileWarns(t *testing.T) {
	t.Chdir("testdata")
	out := filepath.Join(t.TempDir(), "m.csv")
	code, _, errb := runMatrix(t, "inputs/sup.bog", "inputs/jace.bog", t.TempDir(), "-o", out)
	if code != 0 || !strings.Contains(errb, "greenmax-matrix: warning: 0 GreenMAX Px files in ") {
		t.Fatalf("exit %d stderr %q", code, errb)
	}
	got, _ := os.ReadFile(out)
	want, _ := os.ReadFile("csv/matrix-px-empty.csv")
	if !bytes.Equal(got, want) {
		t.Error("the warning must not change the CSV")
	}
	code, _, errb = runMatrix(t, "inputs/sup.bog", "inputs/jace.bog", "inputs/px", "-o", out)
	if code != 0 || strings.Contains(errb, "warning") {
		t.Errorf("with Px files: exit %d stderr %q", code, errb)
	}
}

// An unreadable bog is reported under greenmax-matrix's own name (exit 3, bog.ExitBadInput),
// not as bog-nav.
func TestBogErrorsUseOwnPrefix(t *testing.T) {
	t.Chdir("testdata")
	for _, args := range [][]string{
		{"inputs/nope.bog", "inputs/jace.bog", "inputs/px", "-o", os.DevNull},
		{"inputs/sup.bog", "inputs/not-a-bog.txt", "inputs/px", "-o", os.DevNull},
	} {
		code, _, errb := runMatrix(t, args...)
		if code != 3 || !strings.HasPrefix(errb, "greenmax-matrix: ") || strings.Contains(errb, "bog-nav") {
			t.Errorf("%q: exit %d stderr %q", args, code, errb)
		}
	}
}
