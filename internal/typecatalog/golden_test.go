package typecatalog

import (
	"testing"

	"github.com/angeles725/niagara-go/internal/golden"
)

// TestGolden replays the cases frozen from tools/n4-type-catalog.py (by the retired testdata/freeze_golden.py).
func TestGolden(t *testing.T) { golden.Run(t, "testdata", Main) }
