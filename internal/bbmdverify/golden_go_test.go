package bbmdverify

import (
	"testing"

	"github.com/angeles725/niagara-go/internal/golden"
)

// TestGoldenGo runs the Go-only cases (docs/goldens.md "Go-only cases", D34/D46) against the
// loopback fake: the REBIND NOT CONFIRMED verdict, which the frozen Python never printed (it
// reported REBIND OK whenever a BBMD answered; see golden/bbmd-confirmed-no-cross/intended.json).
//   - forwarded-other-subnet: 127.0.0.7 is a BBMD whose only I-Am is the device's, forwarded
//     from 10.9.9.9, outside the device subnet 127.0.0.4/32: not crossed, exit 1.
//   - bbmd-no-cross-json: the --json form of golden/bbmd-confirmed-no-cross.
//
// GOLDEN_UPDATE=1 rewrites their outputs from Main first.
func TestGoldenGo(t *testing.T) {
	if testing.Short() {
		t.Skip("loopback probe windows take ~1.5 s per case; run without -short (make check)")
	}
	startFake(t)
	golden.RunGo(t, Main)
}
