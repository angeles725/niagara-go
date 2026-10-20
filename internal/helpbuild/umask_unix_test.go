//go:build unix

package helpbuild

import (
	"os"
	"syscall"
	"testing"
)

// TestMain runs the package under umask 022, the umask the frozen Python
// trees were recorded with: treeLines records each path's permission bits,
// and files and folders the builders create take theirs from the umask
// (664/775 under 002). Fixing the umask, rather than masking the recorded
// bits, keeps the explicit modes the tests check (a restored 666
// build.log) exact.
func TestMain(m *testing.M) {
	syscall.Umask(0o022)
	os.Exit(m.Run())
}
