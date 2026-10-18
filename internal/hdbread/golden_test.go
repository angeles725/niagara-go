package hdbread

import (
	"runtime"
	"testing"

	"github.com/angeles725/niagara-go/internal/golden"
)

// TestGolden runs every frozen Python case (usage errors included) through Main.
func TestGolden(t *testing.T) { golden.Run(t, "testdata", Main) }

// TestGoldenLinux runs the cases whose expected text is a Linux errno message (Python on
// Windows words them differently; TestRunDirectoryNamesTheOpenError pins that side). They
// run from testdata (cwd_fixture ".."), like the main suite.
func TestGoldenLinux(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("captured on Linux: open() of a directory is EISDIR there, EACCES on Windows")
	}
	golden.Run(t, "testdata/linux", Main)
}
