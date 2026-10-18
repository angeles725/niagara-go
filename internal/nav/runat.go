package nav

import (
	"path/filepath"

	"github.com/angeles725/niagara-go/internal/cli"
)

// RunAt is Main for the module-navigator checkout navDir, the in-process
// entry for other commands (nsearch): the base directory defaults to navDir
// (an explicit --base-dir in args still wins) and the organized/ corpus is the
// one next to navDir (NAV_CORPUS_BASE still wins), never looked up from
// MODULE_NAVIGATOR_DIR, NAV_DIR, the working directory or the executable. Its
// output is byte-identical to mnav's for the same checkout, and it is safe for
// concurrent calls with different checkouts. An empty navDir is Main.
func RunAt(navDir string, args []string, s cli.Stdio) int {
	if navDir != "" {
		if abs, err := filepath.Abs(navDir); err == nil {
			navDir = abs
		}
	}
	return run(navDir, args, s)
}
