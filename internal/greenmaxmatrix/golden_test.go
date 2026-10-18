package greenmaxmatrix

import (
	"testing"

	"github.com/angeles725/niagara-go/internal/golden"
)

// TestGolden runs every frozen Python case (usage errors included) through Main. The cases that
// write their output to /dev/null require linux: --out /dev/null names no file on Windows.
func TestGolden(t *testing.T) { golden.Run(t, "testdata", Main) }
