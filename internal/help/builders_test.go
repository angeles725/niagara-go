package help

import (
	"strings"
	"testing"

	"github.com/angeles725/niagara-go/internal/cli"
	"github.com/angeles725/niagara-go/internal/helpbuild"
)

// The tests run the maintenance commands as nh does: with the builders
// plugged in, as cmd/nh's main plugs them.
func init() { UseBuilders(helpbuild.NH{}) }

// TestMaintenanceWithoutBuilders: a binary whose main did not plug the
// builders in reports the wiring defect (exit 2), never a silent success.
func TestMaintenanceWithoutBuilders(t *testing.T) {
	saved := builders
	t.Cleanup(func() { UseBuilders(saved) })
	UseBuilders(nil)
	repo := maintRepo(t, emptyInstall(t), false)
	out, errs, code := run(t, repo, "rebuild-all", "--dry-run")
	if code != cli.ExitCannotSearch || !strings.Contains(out+errs, errNoBuilders.Error()) {
		t.Errorf("rebuild-all without builders: exit %d, stdout %q, stderr %q", code, out, errs)
	}
}
