package pxrender

import (
	"testing"

	"github.com/angeles725/niagara-go/internal/golden"
)

// TestGolden runs every parity case (usage errors included) through Main; intended
// differences come from each case's intended.json. The cases that write --out /dev/null
// (and print "/"-joined paths) carry requires.goos ["linux"]: /dev/null names no file on
// Windows, so they are skipped there with that reason.
func TestGolden(t *testing.T) { golden.Run(t, "testdata", Main) }
