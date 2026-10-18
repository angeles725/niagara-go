package pyref

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

// fakeTB records Skip and Fatal instead of stopping the goroutine, so the
// tests see which one the helper chose.
type fakeTB struct {
	testing.TB
	skipped, fataled string
}

func (f *fakeTB) Helper()                  {}
func (f *fakeTB) Skip(a ...any)            { f.skipped = fmt.Sprint(a...) }
func (f *fakeTB) Skipf(s string, a ...any) { f.skipped = fmt.Sprintf(s, a...) }
func (f *fakeTB) Fatal(a ...any)           { f.fataled = fmt.Sprint(a...) }
func (f *fakeTB) Fatalf(s string, a ...any) {
	f.fataled = fmt.Sprintf(s, a...)
}

func git(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cfg := filepath.Join(t.TempDir(), "gitconfig")
	if err := os.WriteFile(cfg, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(gitEnv(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t",
		"GIT_COMMITTER_EMAIL=t@t", "GIT_CONFIG_GLOBAL="+cfg, "GIT_CONFIG_NOSYSTEM=1")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

// tagRepo makes dir a repository with two commits, the first tagged
// pre-go-retire, checked out at the tag; it returns (tag commit, later commit).
func tagRepo(t *testing.T, dir string) (string, string) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
	// As a git hook exports them: neither the helper nor Dir may follow them.
	t.Setenv("GIT_DIR", filepath.Join(t.TempDir(), "elsewhere.git"))
	t.Setenv("GIT_INDEX_FILE", filepath.Join(t.TempDir(), "index"))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	git(t, dir, "init", "-q")
	git(t, dir, "commit", "-q", "--allow-empty", "-m", "frozen")
	git(t, dir, "tag", Tag)
	tag := git(t, dir, "rev-parse", "HEAD")
	git(t, dir, "commit", "-q", "--allow-empty", "-m", "retire")
	later := git(t, dir, "rev-parse", "HEAD")
	git(t, dir, "checkout", "-q", "--detach", Tag)
	return tag, later
}

func TestDirEnvOverride(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "ref")
	tagRepo(t, dir)
	t.Setenv("MODULE_NAVIGATOR_PY_REF", dir)
	t.Setenv(RequiredEnv, "1")
	f := &fakeTB{}
	if got := Dir(f, "module-navigator"); got != dir || f.skipped != "" || f.fataled != "" {
		t.Fatalf("Dir = %q, skip %q, fatal %q; want %q", got, f.skipped, f.fataled, dir)
	}
}

func TestDirDefaultPath(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home) // os.UserHomeDir on Windows
	t.Setenv("NIAGARA_HELP_PY_REF", "")
	want := filepath.Join(home, "niagara-research-worktrees", "pre-go-retire-niagara-help")
	if DefaultDir("niagara-help") != want {
		t.Fatalf("DefaultDir = %q, want %q", DefaultDir("niagara-help"), want)
	}
	tagRepo(t, want)
	f := &fakeTB{}
	if got := Dir(f, "niagara-help"); got != want || f.skipped != "" || f.fataled != "" {
		t.Fatalf("Dir = %q, skip %q, fatal %q; want %q", got, f.skipped, f.fataled, want)
	}
}

func TestDirMissing(t *testing.T) {
	missingDir := filepath.Join(t.TempDir(), "absent")
	t.Setenv("NIAGARA_RESEARCH_PY_REF", missingDir)
	for _, required := range []string{"", "1"} {
		t.Setenv(RequiredEnv, required)
		f := &fakeTB{}
		if got := Dir(f, "niagara-research"); got != "" {
			t.Fatalf("required=%q: Dir = %q, want empty", required, got)
		}
		msg := f.skipped
		if required == "1" {
			msg = f.fataled
			if f.skipped != "" {
				t.Fatalf("required: skipped (%q) instead of failing", f.skipped)
			}
		} else if f.fataled != "" {
			t.Fatalf("not required: failed (%q) instead of skipping", f.fataled)
		}
		if !strings.Contains(msg, missingDir) || !strings.Contains(msg, "NIAGARA_RESEARCH_PY_REF") {
			t.Fatalf("required=%q: message %q names neither the path nor the variable", required, msg)
		}
	}
}

func TestDirHeadMismatchAlwaysFails(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "ref")
	tag, later := tagRepo(t, dir)
	git(t, dir, "checkout", "-q", "--detach", later)
	t.Setenv("MODULE_NAVIGATOR_PY_REF", dir)
	t.Setenv(RequiredEnv, "")
	f := &fakeTB{}
	Dir(f, "module-navigator")
	for _, want := range []string{tag, later, "checkout --detach " + Tag} {
		if !strings.Contains(f.fataled, want) {
			t.Fatalf("fatal %q does not contain %q (skip %q)", f.fataled, want, f.skipped)
		}
	}
}

func TestDirDirtyWorktreeFails(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "ref")
	tagRepo(t, dir)
	if err := os.WriteFile(filepath.Join(dir, "stray.py"), []byte("x = 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("MODULE_NAVIGATOR_PY_REF", dir)
	t.Setenv(RequiredEnv, "")
	f := &fakeTB{}
	if got := Dir(f, "module-navigator"); got != "" || !strings.Contains(f.fataled, "stray.py") {
		t.Fatalf("Dir = %q, fatal %q, skip %q; want a failure naming stray.py", got, f.fataled, f.skipped)
	}
}

func TestDirNotAGitRepoFails(t *testing.T) {
	t.Setenv("MODULE_NAVIGATOR_PY_REF", t.TempDir())
	t.Setenv("GIT_CEILING_DIRECTORIES", os.TempDir())
	f := &fakeTB{}
	Dir(f, "module-navigator")
	if f.fataled == "" {
		t.Fatalf("a directory without the tag did not fail (skip %q)", f.skipped)
	}
}

func TestDirUnknownRepoFails(t *testing.T) {
	f := &fakeTB{}
	Dir(f, "nope")
	if !strings.Contains(f.fataled, "unknown repository") {
		t.Fatalf("fatal = %q", f.fataled)
	}
}

func TestEnv(t *testing.T) {
	for repo, want := range map[string][]string{
		"module-navigator": {"MODULE_NAV_PYTHON=1"},
		"niagara-help":     {"NIAGARA_HELP_PYTHON=1"},
		"niagara-research": nil,
	} {
		if got := Env(repo); !reflect.DeepEqual(got, want) {
			t.Errorf("Env(%q) = %q, want %q", repo, got, want)
		}
	}
}

func TestPython(t *testing.T) {
	defer func(g string) { goos = g }(goos)
	goos = "linux"
	t.Setenv("PATH", t.TempDir())
	for _, required := range []string{"", "1"} {
		t.Setenv(RequiredEnv, required)
		f := &fakeTB{}
		Python(f)
		if (required == "1") != (f.fataled != "") || (required == "") != (f.skipped != "") {
			t.Fatalf("required=%q without python3: skip %q, fatal %q", required, f.skipped, f.fataled)
		}
	}
	bin := t.TempDir()
	py := filepath.Join(bin, "python3")
	if runtime.GOOS == "windows" {
		py += ".exe" // LookPath honours PATHEXT there
	}
	if err := os.WriteFile(py, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)
	f := &fakeTB{}
	if got := Python(f); got != py || f.skipped != "" || f.fataled != "" {
		t.Fatalf("Python = %q, skip %q, fatal %q; want %q", got, f.skipped, f.fataled, py)
	}
	goos = "windows"
	t.Setenv(RequiredEnv, "1")
	f = &fakeTB{}
	if Python(f); f.fataled != "" || !strings.Contains(f.skipped, "Linux only") {
		t.Fatalf("windows, required: skip %q, fatal %q; want a Linux-only skip", f.skipped, f.fataled)
	}
}
