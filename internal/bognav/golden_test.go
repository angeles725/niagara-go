package bognav

import (
	"testing"

	"github.com/angeles725/niagara-go/internal/golden"
)

// TestGolden replays the bog-nav cases of niagara-research (bog-nav.json, converted by
// testdata/convert_golden.py) plus the extra cases frozen from the Python tool.
func TestGolden(t *testing.T) { golden.Run(t, "testdata", Main) }
