package nav

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/angeles725/niagara-go/internal/golden"
)

// Frozen from Python module_nav.py (COLUMNS=100, base dir /nonexistent-base): the
// top-level help, a help-<cmd> sample covering every command group and both nested
// subparsers, the no-command help and usage errors; and, as nav-<case>, the ported
// commands run against the small indexes under testdata/fixture (written by
// gen_golden.py, retired with the Python reference in D47: the cases are frozen
// and change only from reviewed Go output, docs/goldens.md). Cases of commands retired by decision D32 keep Python's
// output and declare the Go result (one "retired" line, exit 2; the shorter
// --help list) in intended.json. The module-navigator golden suite
// (tests/golden_cli.py with GOLDEN_BIN) covers every command against the real corpus.
func TestGolden(t *testing.T) {
	// A checkout gives the drivers and search class indexes and their corpora
	// the same mtime; dated ahead, an index is never "older than the corpus"
	// (no staleness warning), as when the cases were captured.
	future := time.Now().Add(time.Hour)
	for _, base := range []string{"drivers", "search"} {
		ci := filepath.Join("testdata", "fixture", base, "indexes", "class-index.json")
		if err := os.Chtimes(ci, future, future); err != nil {
			t.Fatal(err)
		}
	}
	golden.Run(t, "testdata", Main)
}
