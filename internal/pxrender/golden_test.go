package pxrender

import (
	"testing"

	"github.com/angeles725/niagara-go/internal/golden"
)

// TestGolden runs every parity case (usage errors included) through Main; intended
// differences come from each case's intended.json.
func TestGolden(t *testing.T) { golden.Run(t, "testdata", Main) }
