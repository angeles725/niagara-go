package nav

import (
	"testing"

	"github.com/angeles725/niagara-go/internal/golden"
)

// Frozen from Python module_nav.py (COLUMNS=100, base dir /nonexistent-base): the
// top-level help, a help-<cmd> sample covering every command group and both nested
// subparsers, the no-command help and usage errors. The module-navigator golden suite
// (tests/golden_cli.py with GOLDEN_BIN) covers all 135 help cases.
func TestGolden(t *testing.T) {
	golden.Run(t, "testdata", Main)
}
