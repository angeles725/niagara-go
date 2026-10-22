package greenmaxtablero

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/angeles725/niagara-go/internal/cli"
)

func runMain(args ...string) (code int, stdout, stderr string) {
	var out, errb bytes.Buffer
	code = Main(args, cli.Stdio{Out: &out, Err: &errb})
	return code, out.String(), errb.String()
}

// --gm names a control folder, GM followed by two digits (GM02, GM10): anything else is a usage
// error (exit 2) before the source is read, never ords under Iluminacion/7/.
func TestGMMustBeGMAndTwoDigits(t *testing.T) {
	for _, gm := range []string{"7", "GM7", "gm08", "GM100", "", "GM0A", " GM08"} {
		code, out, errb := runMain("testdata/inputs/tablero.px", "--gm", gm, "--stdout")
		if code != cli.ExitCannotSearch || out != "" || !strings.Contains(errb, "argument --gm:") {
			t.Errorf("--gm %q: exit %d stdout %d bytes stderr %q; want exit 2, usage error", gm, code, len(out), errb)
		}
	}
	code, out, _ := runMain("testdata/inputs/tablero.px", "--gm", "GM08", "--stdout")
	if code != 0 || !strings.Contains(out, "/Iluminacion/GM08/") {
		t.Fatalf("--gm GM08: exit %d", code)
	}
}

// A Px file named after a GreenMAX panel (GreenMAX07 TabAL7.px) that disagrees with --gm warns on
// stderr; the run still succeeds and its output is unchanged.
func TestGMFileNameMismatchWarns(t *testing.T) {
	data, err := os.ReadFile("testdata/inputs/tablero.px")
	if err != nil {
		t.Fatal(err)
	}
	src := filepath.Join(t.TempDir(), "GreenMAX07 TabAL7.px")
	if err := os.WriteFile(src, data, 0o600); err != nil {
		t.Fatal(err)
	}
	code, out, errb := runMain(src, "--gm", "GM02", "--stdout")
	if code != 0 || !strings.Contains(errb, "warning: --gm GM02 does not match GM07") {
		t.Errorf("mismatch: exit %d stderr %q", code, errb)
	}
	code, out2, errb := runMain(src, "--gm", "GM07", "--stdout")
	if code != 0 || strings.Contains(errb, "warning") {
		t.Errorf("match: exit %d stderr %q", code, errb)
	}
	if strings.ReplaceAll(out, "GM02", "GM07") != out2 {
		t.Error("the warning must not change the generated document")
	}
}

// The --gm help agrees with the validation: two digits, leading zero included.
func TestGMHelpIsNotSelfContradictory(t *testing.T) {
	_, out, _ := runMain("--help")
	if strings.Contains(out, "no leading zero") || !strings.Contains(out, "GM02") {
		t.Errorf("--gm help:\n%s", out)
	}
}
