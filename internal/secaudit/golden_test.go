package secaudit

import (
	"testing"

	"github.com/angeles725/niagara-go/internal/golden"
)

// TestGolden runs every frozen Python case (usage errors included) through Main.
func TestGolden(t *testing.T) { golden.Run(t, "testdata", Main) }
