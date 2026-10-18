package course

import (
	"runtime"
	"testing"

	"github.com/angeles725/niagara-go/internal/golden"
)

// The cases run in testdata over the synthetic roots root (every marker and item kind),
// dup-root (parse errors), partial-root (missing sources) and norelated (no labs.md: the bad --root and variable cases).
func TestGolden(t *testing.T) { golden.Run(t, "testdata", Main) }

// TestGoldenLinux runs the cases that print the station's config.bog path joined with "/"
// (the native separator joins it on Windows); they run from testdata (cwd_fixture "..").
func TestGoldenLinux(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("captured on Linux: the config.bog path is joined with the native separator")
	}
	golden.Run(t, "testdata/linux", Main)
}
