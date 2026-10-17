package nav

import (
	"testing"

	"github.com/angeles725/niagara-go/internal/golden"
)

// Frozen from Python module_nav.py (COLUMNS=100, base dir /nonexistent-base): the
// top-level help, a help-<cmd> sample covering every command group and both nested
// subparsers, the no-command help and usage errors; and, as nav-<case>, the ported
// commands run against the small indexes under testdata/fixture (gen_golden.py
// regenerates both). The module-navigator golden suite (tests/golden_cli.py with
// GOLDEN_BIN) covers every command against the real corpus.
func TestGolden(t *testing.T) {
	golden.Run(t, "testdata", Main)
}
