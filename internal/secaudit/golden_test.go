package secaudit

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/angeles725/niagara-go/internal/golden"
)

// TestGolden runs every frozen Python case (usage errors included) through Main.
func TestGolden(t *testing.T) { golden.Run(t, "testdata", Main) }

// TestGoldenUnreadableDir runs the cases that need a log directory nobody can list. Git keeps
// no permissions, so the directory is created here (mode 000, holding an IOC) and removed after.
func TestGoldenUnreadableDir(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("mode 000 does not lock a directory on Windows")
	}
	locked := filepath.Join("testdata", "unreadable-dir", "inputs", "logs-locked", "locked")
	_ = os.Chmod(locked, 0o755) // left behind by an interrupted run
	_ = os.RemoveAll(locked)
	if err := os.Mkdir(locked, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = os.Chmod(locked, 0o755)
		if err := os.RemoveAll(locked); err != nil {
			t.Error(err)
		}
	})
	ioc := []byte("Module validation has been DISABLED\n")
	if err := os.WriteFile(filepath.Join(locked, "station.log"), ioc, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(locked, 0); err != nil {
		t.Fatal(err)
	}
	if _, err := os.ReadDir(locked); err == nil {
		t.Skip("mode 000 does not stop this user listing a directory (root?)")
	}
	golden.Run(t, filepath.Join("testdata", "unreadable-dir"), Main)
}
