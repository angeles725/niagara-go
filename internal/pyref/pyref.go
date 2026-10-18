package pyref

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// Tag is the git tag every reference worktree is checked out at.
const Tag = "pre-go-retire"

// RequiredEnv set to "1" turns a missing reference or python3 from a skip
// into a failure.
const RequiredEnv = "PYREF_REQUIRED"

type repoRef struct{ env, escape string }

var repos = map[string]repoRef{
	"niagara-research": {"NIAGARA_RESEARCH_PY_REF", ""},
	"module-navigator": {"MODULE_NAVIGATOR_PY_REF", "MODULE_NAV_PYTHON=1"},
	"niagara-help":     {"NIAGARA_HELP_PY_REF", "NIAGARA_HELP_PYTHON=1"},
}

// goos is runtime.GOOS; the tests override it.
var goos = runtime.GOOS

// DefaultDir is the reference worktree of repo when its variable is unset:
// $HOME/niagara-research-worktrees/pre-go-retire-<repo>.
func DefaultDir(repo string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "~"
	}
	return filepath.Join(home, "niagara-research-worktrees", Tag+"-"+repo)
}

// Dir is the reference worktree of repo ("niagara-research",
// "module-navigator" or "niagara-help"): its variable, else DefaultDir. A
// missing directory skips t, or fails it when PYREF_REQUIRED=1; a directory
// whose HEAD is not the pre-go-retire commit, or that has local changes or
// untracked files (ignored ones such as __pycache__ pass), always fails t.
func Dir(t testing.TB, repo string) string {
	t.Helper()
	ref, ok := repos[repo]
	if !ok {
		t.Fatalf("pyref: unknown repository %q", repo)
		return ""
	}
	dir := os.Getenv(ref.env)
	if dir == "" {
		dir = DefaultDir(repo)
	}
	if st, err := os.Stat(dir); err != nil || !st.IsDir() {
		missing(t, fmt.Sprintf("%s Python reference not found at %s (set %s or create the worktree: "+
			"git -C <%s checkout> worktree add --detach %s %s)", repo, dir, ref.env, repo, dir, Tag))
		return ""
	}
	head, err := revParse(dir, "HEAD")
	if err != nil {
		t.Fatalf("pyref: %s reference %s: %v", repo, dir, err)
		return ""
	}
	tag, err := revParse(dir, Tag+"^{commit}")
	if err != nil {
		t.Fatalf("pyref: %s reference %s has no tag %s: %v", repo, dir, Tag, err)
		return ""
	}
	if head != tag {
		t.Fatalf("pyref: %s reference %s is at %s, not %s (%s): it is not the frozen Python; "+
			"fix it with: git -C %s checkout --detach %s", repo, dir, head, Tag, tag, dir, Tag)
		return ""
	}
	cmd := exec.Command("git", "-C", dir, "status", "--porcelain")
	cmd.Env = gitEnv()
	if st, err := cmd.Output(); err != nil || len(st) > 0 {
		t.Fatalf("pyref: %s reference %s has local changes or untracked files (%v):\n%s"+
			"it is not the frozen Python; clean it (git -C %s status)", repo, dir, err, st, dir)
		return ""
	}
	return dir
}

// Env is the environment entries that make repo's tag scripts run Python
// rather than an exec shim of the Go port: MODULE_NAV_PYTHON=1,
// NIAGARA_HELP_PYTHON=1, or none for niagara-research.
func Env(repo string) []string {
	if e := repos[repo].escape; e != "" {
		return []string{e}
	}
	return nil
}

// Python is python3 from PATH (no version check). It skips t on every OS but
// Linux, even when PYREF_REQUIRED=1; on Linux a missing python3 skips t, or
// fails it when PYREF_REQUIRED=1.
func Python(t testing.TB) string {
	t.Helper()
	if goos != "linux" {
		t.Skip("Python parity reference runs on Linux only (CRLF/cp1252 on Windows)")
		return ""
	}
	p, err := exec.LookPath("python3")
	if err != nil {
		missing(t, "python3 not on PATH: the Python reference cannot run")
		return ""
	}
	return p
}

func missing(t testing.TB, msg string) {
	t.Helper()
	if os.Getenv(RequiredEnv) == "1" {
		t.Fatal("pyref: " + msg + " (" + RequiredEnv + "=1)")
		return
	}
	t.Skip("pyref: " + msg)
}

func revParse(dir, rev string) (string, error) {
	cmd := exec.Command("git", "-C", dir, "rev-parse", "--verify", "--quiet", rev)
	cmd.Env = gitEnv()
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git rev-parse %s: %v", rev, err)
	}
	return strings.TrimSpace(string(out)), nil
}

// gitEnv is os.Environ without the variables that point git at another
// repository: a git hook (the pre-commit `go test ./...`) exports GIT_DIR and
// GIT_INDEX_FILE, which override -C and would read, or write, the enclosing
// repository instead of dir.
func gitEnv() []string {
	var env []string
	for _, kv := range os.Environ() {
		name, _, _ := strings.Cut(kv, "=")
		switch strings.ToUpper(name) {
		case "GIT_DIR", "GIT_WORK_TREE", "GIT_INDEX_FILE", "GIT_COMMON_DIR", "GIT_OBJECT_DIRECTORY",
			"GIT_ALTERNATE_OBJECT_DIRECTORIES", "GIT_NAMESPACE", "GIT_PREFIX":
			continue
		}
		env = append(env, kv)
	}
	return env
}
