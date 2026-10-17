package course

import (
	"testing"

	"github.com/angeles725/niagara-go/internal/golden"
)

// The cases run in testdata over the synthetic roots root (every marker and item kind),
// dup-root (parse errors), partial-root (missing sources) and norelated (no labs.md: the bad --root and variable cases).
func TestGolden(t *testing.T) { golden.Run(t, "testdata", Main) }
