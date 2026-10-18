package help

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/angeles725/niagara-go/internal/help/scope"
	"github.com/angeles725/niagara-go/internal/helpbuild"
)

// rebuild-all runs helpbuild.RebuildAll in process under the build lock
// (cli/cmd_admin.py run_rebuild_all). The bytes of the plan and the run are
// pinned against Python by helpbuild's rebuild parity and, through nh, by
// TestParityRebuildReal; these tests pin the wiring: the flags, the exit
// statuses and the lock.

const manifestRel = "indexes/.manifest.json"

func noLockLeft(t *testing.T, repo string) {
	t.Helper()
	if _, err := os.Stat(scope.BuildLockPath(repo)); !os.IsNotExist(err) {
		t.Errorf("build lock left behind: %v", err)
	}
}

func TestRebuildAllDryRun(t *testing.T) {
	repo := maintRepo(t, emptyInstall(t), false)
	out, errs, code := run(t, repo, "rebuild-all", "--dry-run")
	var want strings.Builder
	if _, err := helpbuild.RebuildAll(repo, helpbuild.RebuildOptions{DryRun: true, Stdout: &want}); err != nil {
		t.Fatal(err)
	}
	if code != 0 || errs != "" || out != want.String() ||
		!strings.HasSuffix(out, "\nDry run — nothing executed. Drop --dry-run to build.\n") {
		t.Errorf("code %d stderr %q stdout:\n%s\nwant:\n%s", code, errs, out, want.String())
	}
	if _, err := os.Stat(filepath.Join(repo, manifestRel)); !os.IsNotExist(err) {
		t.Errorf("a dry run wrote the manifest: %v", err)
	}
	noLockLeft(t, repo)
}

func TestRebuildAllOnlyNothingStamps(t *testing.T) {
	repo := maintRepo(t, emptyInstall(t), false)
	out, errs, code := run(t, repo, "rebuild-all", "--only", "zzz")
	want := "REBUILD-ALL \n" + strings.Repeat("=", 60) + "\n\nRan 0, skipped 0, failed 0.\n" +
		"Manifest re-stamped. Run 'freshness' to confirm FRESH.\n"
	if code != 0 || errs != "" || out != want {
		t.Errorf("code %d stderr %q stdout:\n%s", code, errs, out)
	}
	if _, err := os.Stat(filepath.Join(repo, manifestRel)); err != nil {
		t.Errorf("manifest not stamped: %v", err)
	}
	noLockLeft(t, repo)
}

func TestRebuildAllStampOnly(t *testing.T) {
	stamped := regexp.MustCompile(`^Manifest stamped at \d{4}-\d\d-\d\dT[0-9:.]+\+00:00 \(baseline = current corpus\)\.\n$`)
	for _, args := range [][]string{{"rebuild-all", "--stamp-only"}, {"rebuild-all", "--dry-run", "--stamp-only"}} {
		repo := maintRepo(t, emptyInstall(t), false)
		out, errs, code := run(t, repo, args...)
		if code != 0 || errs != "" || !stamped.MatchString(out) {
			t.Errorf("%v: code %d stderr %q stdout %q", args, code, errs, out)
		}
		if _, err := os.Stat(filepath.Join(repo, manifestRel)); err != nil {
			t.Errorf("%v: manifest not stamped: %v", args, err)
		}
		noLockLeft(t, repo)
	}
}

// TestRebuildAllTakesOverAStaleLock: the rebuild uses the full-text build
// lock: a lock left by a crashed process is taken over and removed.
func TestRebuildAllTakesOverAStaleLock(t *testing.T) {
	repo := maintRepo(t, emptyInstall(t), false)
	lock := scope.BuildLockPath(repo)
	write(t, lock, "1 dead\n")
	old := time.Now().Add(-time.Hour)
	if err := os.Chtimes(lock, old, old); err != nil {
		t.Fatal(err)
	}
	out, errs, code := run(t, repo, "rebuild-all", "--stamp-only")
	if code != 0 || errs != "" || !strings.HasPrefix(out, "Manifest stamped at ") {
		t.Errorf("code %d stderr %q stdout %q", code, errs, out)
	}
	noLockLeft(t, repo)
}

// TestRebuildAllUnwritableIndexes: a lock that cannot be created is an
// ERROR line and exit 2, and nothing runs.
func TestRebuildAllUnwritableIndexes(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root writes into a read-only folder")
	}
	repo := maintRepo(t, emptyInstall(t), false)
	indexes := filepath.Join(repo, "indexes")
	if err := os.Chmod(indexes, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(indexes, 0o755) })
	out, errs, code := run(t, repo, "rebuild-all", "--only", "zzz")
	if code != 2 || out != "" || !strings.HasPrefix(errs, "ERROR: open "+scope.BuildLockPath(repo)) {
		t.Errorf("code %d stdout %q stderr %q", code, out, errs)
	}
}

func TestRebuildAllHelp(t *testing.T) {
	repo := maintRepo(t, emptyInstall(t), false)
	out, errs, code := run(t, repo, "rebuild-all", "--help")
	for _, want := range []string{"usage: niagara_help rebuild-all [-h] [--only ONLY] [--dry-run] [--stamp-only]",
		"Only run builders whose script starts with this", "Print the build plan without executing",
		"Record current corpus as the baseline without rebuilding"} {
		if !strings.Contains(out, want) {
			t.Errorf("rebuild-all --help lacks %q:\n%s", want, out)
		}
	}
	if code != 0 || errs != "" {
		t.Errorf("code %d stderr %q", code, errs)
	}
	top, _, _ := run(t, repo, "--help")
	if !strings.Contains(top, "    rebuild-all         Rebuild all indexes in dependency order, then stamp the manifest") {
		t.Errorf("--help does not list rebuild-all:\n%s", top)
	}
}
