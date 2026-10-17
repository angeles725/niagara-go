package modulefind

import (
	"testing"

	"github.com/angeles725/niagara-go/internal/golden"
)

// The cases are a subset of niagara-research tools/tests/fixtures/golden-cli/module-find.json
// (the Python output), run in testdata with the roots srcA, srcB and nojava relative to it.
func TestGolden(t *testing.T) { golden.Run(t, "testdata", Main) }
