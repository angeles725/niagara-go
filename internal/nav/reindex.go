package nav

import (
	"io"

	"github.com/angeles725/niagara-go/internal/argparse"
	"github.com/angeles725/niagara-go/internal/cli"
)

// reindexArgs recognises an mnav reindex line, module-navigator's reindex.sh
// (not a module_nav.py command, so not in the registry, the --help list or
// Order): "reindex" as commandWord. The global options before it (and before
// a "--" that ends them) are parsed as module-nav parses them, and their
// --base-dir is passed on to the rebuild ahead of its own arguments. A global
// option the parser rejects leaves the line to module-nav, which reports it.
func reindexArgs(args []string) ([]string, bool) {
	i := commandIndex(args)
	if i < 0 || args[i] != "reindex" {
		return nil, false
	}
	globals := args[:i]
	if i > 0 && args[i-1] == "--" {
		globals = args[:i-1]
	}
	p := argparse.New(argparse.Config{Prog: "module-nav"})
	addGlobalOptions(p)
	ns, _, done := p.Parse(globals, cli.Stdio{Out: io.Discard, Err: io.Discard})
	if done {
		return nil, false
	}
	rest := args[i+1:]
	if dir, ok := ns.Get("base_dir").(string); ok {
		rest = append([]string{"--base-dir", dir}, rest...)
	}
	return rest, true
}
