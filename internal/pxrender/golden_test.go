package pxrender

import (
	"testing"

	"github.com/angeles725/niagara-go/internal/golden"
)

// TestGolden runs every parity case (usage errors included) through Main; intended
// differences come from each case's intended.json. The cases that write --out /dev/null
// (and print "/"-joined paths) carry requires.goos ["linux"]: /dev/null names no file on
// Windows, so they are skipped there with that reason. $NIAGARA_ORGANIZED is emptied so a
// caller's value cannot give the cases without --organized an organized/ tree.
func TestGolden(t *testing.T) {
	t.Setenv("NIAGARA_ORGANIZED", "")
	golden.Run(t, "testdata", Main)
}
