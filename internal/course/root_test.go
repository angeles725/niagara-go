package course

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/angeles725/niagara-go/internal/cli"
	"github.com/angeles725/niagara-go/internal/course/key"
)

// mkRoot creates dir with an empty labs.md marker.
func mkRoot(t *testing.T, dir string) string {
	t.Helper()
	p := filepath.Join(dir, filepath.FromSlash(key.LabsFile))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

// isolate runs the test from an empty temporary directory with the executable stubbed into
// another one, so no labs.md above the real working directory or test binary can leak in.
func isolate(t *testing.T, exeDir string) {
	t.Helper()
	t.Chdir(t.TempDir())
	old := executable
	executable = func() (string, error) { return filepath.Join(exeDir, "course"), nil }
	t.Cleanup(func() { executable = old })
	t.Setenv(RootEnv, "")
}

func TestFindRootOrder(t *testing.T) {
	base := t.TempDir()
	flag := mkRoot(t, filepath.Join(base, "flag"))
	env := mkRoot(t, filepath.Join(base, "env"))
	cwd := mkRoot(t, filepath.Join(base, "cwd"))
	exe := mkRoot(t, filepath.Join(base, "exe"))
	empty := filepath.Join(base, "empty")
	for _, d := range []string{filepath.Join(cwd, "a", "b"), filepath.Join(exe, "bin"), empty} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	isolate(t, filepath.Join(exe, "bin"))

	expect := func(label, got string, err error, want string) {
		t.Helper()
		if err != nil || got != want {
			t.Errorf("%s: findRoot = %q, %v; want %q", label, got, err, want)
		}
	}
	// Nothing at the cwd: the walk from the executable's directory finds exe.
	got, err := findRoot("", false)
	expect("executable walk", got, err, exe)

	// The walk from the working directory beats the executable.
	t.Chdir(filepath.Join(cwd, "a", "b"))
	got, err = findRoot("", false)
	expect("cwd walk", got, err, cwd)

	// The variable beats the walks, and is used trimmed.
	t.Setenv(RootEnv, "  "+env+"\t")
	got, err = findRoot("", false)
	expect("env (trimmed)", got, err, env)

	// --root beats the variable.
	got, err = findRoot(flag, true)
	expect("flag", got, err, flag)

	// A named directory without labs.md is an error, never a fallback.
	if _, err := findRoot(empty, true); err == nil {
		t.Errorf("--root without labs.md: no error")
	}
	t.Setenv(RootEnv, empty)
	if _, err := findRoot("", false); err == nil {
		t.Errorf("%s without labs.md: no error", RootEnv)
	}
}

// TestMainNoRoot replaces the old root-none golden, which depended on no labs.md existing
// above the test's working directory or binary.
func TestMainNoRoot(t *testing.T) {
	isolate(t, t.TempDir())
	var out, errb bytes.Buffer
	code := Main([]string{"show", "L1.1"}, cli.Stdio{In: bytes.NewReader(nil), Out: &out, Err: &errb})
	want := "course: cannot find docs/n4-dev-course-solutions/labs.md: pass --root DIR, set " +
		"NIAGARA_RESEARCH_DIR, or run inside a niagara-research checkout\n"
	if code != cli.ExitCannotSearch || out.Len() != 0 || errb.String() != want {
		t.Errorf("exit %d, stdout %q, stderr %q; want 2, empty, %q", code, out.String(), errb.String(), want)
	}
}
