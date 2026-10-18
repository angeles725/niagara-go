package searchdoor

import (
	"bytes"
	"strings"

	"github.com/angeles725/niagara-go/internal/cli"
	"github.com/angeles725/niagara-go/internal/nav"
)

// Every other module_nav call (the structure commands hierarchy, implementors and
// drivers, the `drivers --help` probe, modules and the license-feature(s) of the
// license join), and every token, grep or rank call its fast path leaves, runs
// module-nav's own command in process through nav.RunAt (decision D42): the same
// bytes mnav prints, for the checkout of the script.

// navRun is module_nav <args> through nav.RunAt on the checkout module_nav.py
// would use (navDir). A script without indexes/ beside its tools/ folder is
// module_nav's "Cannot detect base directory" (commands/__init__.py main, exit 1):
// RunAt pins a checkout, so it has no such case of its own.
func navRun(argv []string) answer {
	base := navDir(argv[0])
	if base == "" {
		return exact(1, "ERROR: Cannot detect base directory.\nUse --base-dir or run from module-navigator/tools/\n", "")
	}
	var out, errb bytes.Buffer
	rc := nav.RunAt(base, argv[1:], cli.Stdio{In: strings.NewReader(""), Out: &out, Err: &errb})
	return exact(rc, out.String(), errb.String())
}

// orNav is a fast path with navRun behind it for the calls it leaves.
func orNav(fast func([]string) answer) func([]string) answer {
	return func(argv []string) answer {
		if a := fast(argv); a.ok {
			return a
		}
		return navRun(argv)
	}
}
