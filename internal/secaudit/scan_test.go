package secaudit

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/angeles725/niagara-go/internal/cli"
)

// lockedDir is a directory under a temp root that this user cannot list (removed after the test).
func lockedDir(t *testing.T, root, name string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("mode 000 does not lock a directory on Windows")
	}
	d := filepath.Join(root, name)
	if err := os.Mkdir(d, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(d, 0o755) })
	if _, err := os.ReadDir(d); err == nil {
		t.Skip("mode 000 does not stop this user listing a directory (root?)")
	}
	return d
}

// An unlistable root scans nothing: an error that counts it, never a clean verdict.
func TestScanLogsUnreadableRoot(t *testing.T) {
	root := lockedDir(t, t.TempDir(), "logs")
	var out, errb bytes.Buffer
	code := runScanLogs(root, false, cli.Stdio{Out: &out, Err: &errb})
	want := "WARNING: --scan-logs: 1 unreadable directory(ies) not scanned:\n  " + root + ": Permission denied\n" +
		"ERROR: --scan-logs: 0 readable log files (*.log, console/station/nre/daemon *.txt) under " + root + " (1 unreadable)\n"
	if code != cli.ExitCannotSearch || out.Len() != 0 || errb.String() != want {
		t.Errorf("exit %d, stdout %q, stderr %q", code, out.String(), errb.String())
	}
}

// Hits found next to an unlistable directory are still printed; the exit says incomplete.
func TestScanLogsHitsBesideAnUnreadableDir(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "nre.log"), []byte("No code signers for entry x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	lockedDir(t, root, "old")
	hits, nfiles, bad, badDirs := scanLogs(root)
	if len(hits) != 1 || nfiles != 1 || len(bad) != 0 || len(badDirs) != 1 || badDirs[0].path != root+"/old" {
		t.Fatalf("hits %v nfiles %d bad %v badDirs %v", hits, nfiles, bad, badDirs)
	}
	var out, errb bytes.Buffer
	if code := runScanLogs(root, true, cli.Stdio{Out: &out, Err: &errb}); code != cli.ExitCannotSearch {
		t.Errorf("exit %d, want %d (incomplete)", code, cli.ExitCannotSearch)
	}
}
