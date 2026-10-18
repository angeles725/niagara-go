package modulefind

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/angeles725/niagara-go/internal/golden"
)

// The cases are niagara-research tools/tests/fixtures/golden-cli/module-find.json (the Python
// output, converted by testdata/convert_golden.py) plus Go-only --limit cases, run in testdata
// with the roots srcA, srcB and nojava relative to it.
func TestGolden(t *testing.T) { golden.Run(t, "testdata", Main) }

// TestGoldenUnreadable runs module-find-unreadable.json like the Python suite: in a temporary
// directory holding src/, a copy of srcA whose com/acme/util/Helper.java is mode 000.
func TestGoldenUnreadable(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("needs a mode-000 file the test user cannot read")
	}
	dir := t.TempDir()
	if err := os.CopyFS(filepath.Join(dir, "src"), os.DirFS(filepath.Join("testdata", "srcA"))); err != nil {
		t.Fatal(err)
	}
	if err := os.CopyFS(filepath.Join(dir, "golden"), os.DirFS(filepath.Join("testdata", "unreadable", "golden"))); err != nil {
		t.Fatal(err)
	}
	locked := filepath.Join(dir, "src", "com", "acme", "util", "Helper.java")
	if err := os.Chmod(locked, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(locked, 0o644) })
	golden.Run(t, dir, Main)
}
