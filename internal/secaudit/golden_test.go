package secaudit

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/angeles725/niagara-go/internal/golden"
)

// TestGolden runs every frozen Python case (usage errors included) through Main. The
// scan-logs cases that print a log path joined under the --scan-logs directory carry
// requires.goos ["linux"]: they were captured with posixpath.join, and on Windows
// os.path.join puts a backslash there (TestScanLogsNamesFilesWithTheHostSeparator).
func TestGolden(t *testing.T) { golden.Run(t, "testdata", Main) }

// TestGoldenDanglingLinks runs the cases whose log directories hold a symlink to a missing file
// (inputs/logs-dangling/nre.log and inputs/logs-partial/station.log, both -> gone.log). The
// links are made in a temp copy of the suite, so the checked-in tree holds none (a Windows
// checkout cannot create them). The frozen paths are joined with "/" and the reason is the
// POSIX ENOENT text, so every POSIX host reproduces them byte for byte; only Windows (which
// joins with a backslash and needs a privilege to make symlinks) is left out.
func TestGoldenDanglingLinks(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("frozen on POSIX: Windows joins the unreadable log path with a backslash")
	}
	suite := t.TempDir()
	if err := os.CopyFS(suite, os.DirFS(filepath.Join("testdata", "dangling-links"))); err != nil {
		t.Fatal(err)
	}
	for _, link := range []string{"logs-dangling/nre.log", "logs-partial/station.log"} {
		path := filepath.Join(suite, "inputs", filepath.FromSlash(link))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink("gone.log", path); err != nil {
			t.Fatal(err)
		}
	}
	golden.Run(t, suite, Main)
}

// TestGoldenUnreadableDir runs the cases that need a log directory nobody can list. Git keeps
// no permissions, so the suite is copied under a temp root and the directory (mode 000, holding
// an IOC) is created there: the checked-in tree is never written, and a killed run leaves
// nothing behind in it.
func TestGoldenUnreadableDir(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("mode 000 does not lock a directory on Windows")
	}
	suite := t.TempDir()
	if err := os.CopyFS(suite, os.DirFS(filepath.Join("testdata", "unreadable-dir"))); err != nil {
		t.Fatal(err)
	}
	locked := filepath.Join(suite, "inputs", "logs-locked", "locked")
	if err := os.Mkdir(locked, 0o755); err != nil {
		t.Fatal(err)
	}
	ioc := []byte("Module validation has been DISABLED\n")
	if err := os.WriteFile(filepath.Join(locked, "station.log"), ioc, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(locked, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(locked, 0o755) }) // runs before TempDir removes the suite
	if _, err := os.ReadDir(locked); err == nil {
		t.Skip("mode 000 does not stop this user listing a directory (root?)")
	}
	golden.Run(t, suite, Main)
}
