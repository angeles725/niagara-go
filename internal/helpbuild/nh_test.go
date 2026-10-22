package helpbuild

import (
	"testing"

	"github.com/angeles725/niagara-go/internal/help/buildapi"
)

// TestNHGateStatuses: the gate statuses nh's reader compares against
// (buildapi) are the ones the sweep and the doc-jar gate return.
func TestNHGateStatuses(t *testing.T) {
	if buildapi.SweepPass != SweepPass || buildapi.SweepUnavailable != SweepUnavailable {
		t.Errorf("buildapi statuses %d/%d, helpbuild %d/%d", buildapi.SweepPass, buildapi.SweepUnavailable,
			SweepPass, SweepUnavailable)
	}
}
