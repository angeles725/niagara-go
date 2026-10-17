package golden

import (
	"bytes"
	"fmt"
	"io"
	"os"
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
	os.WriteFile(filepath.Join(c, "cmd.json"), []byte(`{"argv":["x"]}`), 0o644)
	os.WriteFile(filepath.Join(c, "stdout"), nil, 0o644)
	os.WriteFile(filepath.Join(c, "exit"), []byte("0"), 0o644)
	if _, err := Load(dir); err == nil {
		t.Fatal("cmd.json with unknown field accepted")
	}
}

func TestLoadRejectsEmptySuite(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "golden"), 0o755)
	if _, err := Load(dir); err == nil {
		t.Fatal("empty suite accepted: a missing fixture set must not pass silently")
	}
}

func TestDiffNamesFirstLine(t *testing.T) {
	d := Diff([]byte("a\nb\n"), []byte("a\nc\n"))
	if !strings.Contains(d, "line 2") || !bytes.Contains([]byte(d), []byte(`"c"`)) {
		t.Fatalf("diff = %q", d)
	}
}

type fakeT struct{ errs []string }

func (f *fakeT) Helper() {}
func (f *fakeT) Errorf(format string, args ...any) {
	f.errs = append(f.errs, fmt.Sprintf(format, args...))
}
