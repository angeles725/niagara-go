// Command mnav is module-nav, the Niagara N4 decompiled-module navigator.
package main

import (
	"strings"

	"github.com/angeles725/niagara-go/internal/cli"
	"github.com/angeles725/niagara-go/internal/nav"
	"github.com/angeles725/niagara-go/internal/navbuild/reindex"
)

func main() { cli.Run(run) }

// run dispatches `mnav [--base-dir DIR] reindex ...` to the index rebuild
// (module-navigator's reindex.sh, not a module_nav.py command, so it is not
// in the command registry or the --help list) and everything else to nav.
func run(args []string, s cli.Stdio) int {
	if rest, ok := reindexArgs(args); ok {
		return reindex.Main(rest, s)
	}
	return nav.Main(args, s)
}

// reindexArgs recognises a reindex command line: "reindex" as the first
// argument, or after a leading --base-dir DIR / -d DIR / --base-dir=DIR,
// which is then passed on to it.
func reindexArgs(args []string) ([]string, bool) {
	switch {
	case len(args) >= 1 && args[0] == "reindex":
		return args[1:], true
	case len(args) >= 3 && (args[0] == "--base-dir" || args[0] == "-d") && args[2] == "reindex":
		return append([]string{"--base-dir", args[1]}, args[3:]...), true
	case len(args) >= 2 && strings.HasPrefix(args[0], "--base-dir=") && args[1] == "reindex":
		return append([]string{"--base-dir", strings.TrimPrefix(args[0], "--base-dir=")}, args[2:]...), true
	}
	return nil, false
}
