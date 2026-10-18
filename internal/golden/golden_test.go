package golden

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
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
	case "--crash":
		fmt.Fprintln(s.Err, "toy: boom")
		return cli.ExitCannotSearch
	case "--tmp":
		fmt.Fprintf(s.Out, "wrote %s\n", args[1])
		return cli.ExitFound
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

func TestIntendedRequiresReasonAndAnOverride(t *testing.T) {
	for name, body := range map[string]string{
		"no-reason": `{"exit": 2}`,
		"blank":     `{"reason": "  ", "exit": 2}`,
		"no-change": `{"reason": "x"}`,
		"bad-field": `{"reason": "x", "code": 2}`,
	} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			c := filepath.Join(dir, "golden", "case")
			if err := os.MkdirAll(c, 0o755); err != nil {
				t.Fatal(err)
			}
			write(t, filepath.Join(c, "cmd.json"), `{"args":[]}`)
			write(t, filepath.Join(c, "stdout"), "")
			write(t, filepath.Join(c, "exit"), "1")
			write(t, filepath.Join(c, "intended.json"), body)
			if _, err := Load(dir); err == nil {
				t.Fatalf("intended.json %s accepted", body)
			}
		})
	}
}

func TestIntendedOverridesThePythonExpectation(t *testing.T) {
	cases, err := Load("testdata")
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range cases {
		if c.Name != "intended-crash" {
			continue
		}
		if c.Intended == nil || c.Exit != 2 || string(c.Stderr) != "toy: boom\n" {
			t.Fatalf("override not applied: %+v", c)
		}
		return
	}
	t.Fatal("intended-crash case missing")
}

func TestHostBoundCaseRunsOnlyOnItsHost(t *testing.T) {
	f := filepath.Join(t.TempDir(), "machine-id")
	write(t, f, "abc\n")
	old := MachineIDFile
	MachineIDFile = f
	defer func() { MachineIDFile = old }()
	c := Case{Requires: Requires{MachineIDSHA256: "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad"}}
	var skippedOwn bool
	t.Run("matching", func(t *testing.T) {
		defer func() { skippedOwn = t.Skipped() }()
		skipUnlessHost(t, c)
	})
	if skippedOwn {
		t.Fatal("case skipped on its own host")
	}
	var skipped bool
	t.Run("other", func(t *testing.T) {
		defer func() { skipped = t.Skipped() }()
		skipUnlessHost(t, Case{Requires: Requires{MachineIDSHA256: "00"}})
	})
	if !skipped {
		t.Fatal("host-bound case was not skipped on another host")
	}
}

func TestOSBoundCaseRunsOnlyOnListedOS(t *testing.T) {
	old := hostGOOS
	hostGOOS = "windows"
	defer func() { hostGOOS = old }()
	for _, tt := range []struct {
		name     string
		goos     []string
		wantSkip bool
	}{
		{"no requirement", nil, false},
		{"listed", []string{"linux", "windows"}, false},
		{"not listed", []string{"linux", "darwin"}, true},
	} {
		var skipped bool
		t.Run(tt.name, func(t *testing.T) {
			defer func() { skipped = t.Skipped() }()
			skipUnlessHost(t, Case{Requires: Requires{GOOS: tt.goos}})
		})
		if skipped != tt.wantSkip {
			t.Errorf("%s: skipped = %v, want %v", tt.name, skipped, tt.wantSkip)
		}
	}
}

func TestRequiresGOOSIsValidatedOnLoad(t *testing.T) {
	for name, tt := range map[string]struct {
		requires string
		wantErr  string
	}{
		"empty list": {`{"goos": []}`, "never run"},
		"typo":       {`{"goos": ["linux", "window"]}`, `unknown GOOS "window"`},
		"valid":      {`{"goos": ["linux", "windows"]}`, ""},
	} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			c := filepath.Join(dir, "golden", "case")
			if err := os.MkdirAll(c, 0o755); err != nil {
				t.Fatal(err)
			}
			write(t, filepath.Join(c, "cmd.json"), `{"args":[], "requires": `+tt.requires+`}`)
			write(t, filepath.Join(c, "stdout"), "")
			write(t, filepath.Join(c, "exit"), "0")
			cases, err := Load(dir)
			switch {
			case tt.wantErr == "" && err != nil:
				t.Fatalf("Load: %v", err)
			case tt.wantErr == "" && !slices.Equal(cases[0].Requires.GOOS, []string{"linux", "windows"}):
				t.Fatalf("goos = %v", cases[0].Requires.GOOS)
			case tt.wantErr != "" && (err == nil || !strings.Contains(err.Error(), tt.wantErr)):
				t.Fatalf("Load error = %v, want %q", err, tt.wantErr)
			}
		})
	}
}

func TestTmpIsFoldedBackOnlyWhenUsed(t *testing.T) {
	c, tmp := bind(t, Case{Args: []string{"x"}})
	if tmp != "" || c.Args[0] != "x" {
		t.Fatalf("bind without placeholder changed the case: %v %q", c.Args, tmp)
	}
	c, tmp = bind(t, Case{Args: []string{"{tmp}/a"}, Env: map[string]string{"D": "{tmp}"}})
	if tmp == "" || c.Args[0] != tmp+"/a" || c.Env["D"] != tmp {
		t.Fatalf("bind = %v %v %q", c.Args, c.Env, tmp)
	}
	r := unbind(result{stdout: []byte(tmp + "/a\n")}, tmp)
	if string(r.stdout) != "{tmp}/a\n" {
		t.Fatalf("unbind = %q", r.stdout)
	}
}
