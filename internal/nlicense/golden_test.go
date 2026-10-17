package nlicense

import (
	"testing"

	"github.com/angeles725/niagara-go/internal/golden"
)

// Golden cases were frozen from tools/niagara-license-tool.py (Python 3.15,
// cryptography 50.0.2, lxml 6.1.3 through uv) with COLUMNS=100 NO_COLOR=1,
// cwd = testdata.
func TestGolden(t *testing.T) { golden.Run(t, "testdata", Main) }
