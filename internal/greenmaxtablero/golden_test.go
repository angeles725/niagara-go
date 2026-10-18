package greenmaxtablero

import (
	"runtime"
	"testing"

	"github.com/angeles725/niagara-go/internal/golden"
)

// TestGolden runs every frozen Python case (usage errors included) through Main.
func TestGolden(t *testing.T) { golden.Run(t, "testdata", Main) }

// TestGoldenLinux runs the cases that write their output to /dev/null; they run from testdata
// (cwd_fixture ".."), like the main suite.
func TestGoldenLinux(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("captured on Linux: --out /dev/null names no file on Windows")
	}
	golden.Run(t, "testdata/linux", Main)
}
