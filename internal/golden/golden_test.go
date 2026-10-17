package golden

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/angeles725/niagara-go/internal/cli"
)

// toy behaves like a tiny tool so the runner itself is exercised.
func toy(args []string, s cli.Stdio) int {
	switch args[0] {
	case "--stdin":
		data, _ := io.ReadAll(s.In)
		fmt.Fprintf(s.Err, "no match in %d bytes\n", len(data))
		return cli.ExitNoMatch
	case "--env":
		wd, _ := os.Getwd()
		fmt.Fprintf(s.Out, "COLUMNS=%s NO_COLOR=%s cwd=%s\n", os.Getenv("COLUMNS"), os.Getenv("NO_COLOR"), filepath.Base(wd))
		return cli.ExitFound
	}
	fmt.Fprintln(s.Out, strings.Join(args, " "))
	return cli.ExitFound
}

func TestRunPassesMatchingCases(t *testing.T) {
	Run(t, "testdata", toy)
}

func TestCompareReportsEveryMismatch(t *testing.T) {
	c := Case{Stdout: []byte("a\nb\n"), Stderr: nil, Exit: 0}
	diffs := 0
	check := func(got result) {
		var ft fakeT
		compareInto(&ft, c, got)
		diffs = len(ft.errs)
	}
	check(result{stdout: []byte("a\nX\n"), stderr: []byte("e"), exit: 2})
	if diffs != 3 {
		t.Fatalf("want 3 mismatches (exit, stdout, stderr), got %d", diffs)
	}
	check(result{stdout: []byte("a\nb\n"), exit: 0})
	if diffs != 0 {
		t.Fatalf("identical result reported %d mismatches", diffs)
	}
}

func TestLoadRejectsUnknownFields(t *testing.T) {
	dir := t.TempDir()
	c := filepath.Join(dir, "golden", "bad")
	if err := os.MkdirAll(c, 0o755); err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(c, "cmd.json"), `{"argv":["x"]}`)
	write(t, filepath.Join(c, "stdout"), "")
	write(t, filepath.Join(c, "exit"), "0")
	if _, err := Load(dir); err == nil || !strings.Contains(err.Error(), "argv") {
		t.Fatal("cmd.json with unknown field accepted")
	}
}

func TestLoadRejectsEmptySuite(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "golden"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(dir); err == nil {
		t.Fatal("empty suite accepted: a missing fixture set must not pass silently")
	}
}

func TestDiffNamesFirstLine(t *testing.T) {
	d := Diff([]byte("a\nb\n"), []byte("a\nc\n"))
	if !strings.Contains(d, "at line 2") || !bytes.Contains([]byte(d), []byte(`"c"`)) {
		t.Fatalf("diff = %q", d)
	}
}

type fakeT struct{ errs []string }

func (f *fakeT) Helper() {}
func (f *fakeT) Errorf(format string, args ...any) {
	f.errs = append(f.errs, fmt.Sprintf(format, args...))
}

func write(t *testing.T, path, data string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestExecModeRunsBinary builds the toy as a real binary and runs the same
// cases through GOLDEN_BIN with a relative path.
func TestExecModeRunsBinary(t *testing.T) {
	if testing.Short() {
		t.Skip("builds a binary")
	}
	bin := filepath.Join(t.TempDir(), "toy")
	build := exec.Command("go", "build", "-o", bin, "./testdata/toy")
	build.Env = append(os.Environ(), "CGO_ENABLED=0")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build toy: %v\n%s", err, out)
	}
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	rel, err := filepath.Rel(wd, bin)
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv(BinEnv, rel)
	Run(t, "testdata", nil)
}
