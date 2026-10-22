package modulefind

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// A fixture the selftest cannot write fails it with the cause, instead of FAIL lines that read
// as parser regressions.
func TestSelftestReportsUnwritableFixture(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("a read-only directory mode does not stop this user's writes here")
	}
	ro := filepath.Join(t.TempDir(), "ro")
	if err := os.Mkdir(ro, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(ro, 0o700) })
	old := selftestDir
	selftestDir = func() (string, error) { return ro, nil }
	t.Cleanup(func() { selftestDir = old })
	var out bytes.Buffer
	if code := selftest(&out); code != 1 {
		t.Fatalf("selftest = %d, want 1; output:\n%s", code, out.String())
	}
	if !strings.Contains(out.String(), "SELFTEST FAILED: cannot write the fixture ") ||
		!strings.Contains(out.String(), "permission denied") {
		t.Fatalf("selftest did not report the write error:\n%s", out.String())
	}
}
