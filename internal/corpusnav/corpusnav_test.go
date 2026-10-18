package corpusnav

import (
	"testing"

	"github.com/angeles725/niagara-go/internal/golden"
)

// The cases are every case of niagara-research tools/tests/fixtures/golden-cli/corpus-nav.json
// (the Python output, converted by testdata/convert_golden.py) plus Go-only --limit cases, run
// in testdata with CORPUS_NAV_ROOT=corpus and the find cache off (the Python golden pins the
// same find output with and without its index).
func TestGolden(t *testing.T) { golden.Run(t, "testdata", Main) }
