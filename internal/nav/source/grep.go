package source

import (
	"fmt"

	"github.com/angeles725/niagara-go/internal/argparse"
	"github.com/angeles725/niagara-go/internal/nav/lib/codesearch"
	"github.com/angeles725/niagara-go/internal/nav/registry"
	"github.com/angeles725/niagara-go/internal/pyre"
)

// runGrep is commands/source.py run_grep (grep_search.cmd_grep).
func runGrep(env *registry.Env, ns *argparse.Namespace) int {
	pattern, limit := ns.String("pattern"), ns.Int("n")
	f := codesearch.GrepFilter{Sub: ns.String("grep_module"), Type: ns.String("grep_type")}
	re, err := pyre.Compile(pattern, 0)
	if err != nil {
		env.Error(fmt.Sprintf("invalid regex '%s': %v", pattern, err), "")
		return env.ExitCode()
	}
	organized, tried := env.ResolveOrganized()
	if organized == "" {
		env.Error("corpus not found: "+tried, "Set NAV_CORPUS_BASE to the organized/ directory.")
		return env.ExitCode()
	}
	res := codesearch.Grep(organized, re, limit, f)
	out := env.Stdio.Out
	if len(res.Hits) == 0 {
		env.Miss("/"+pattern+"/", codesearch.GrepScope(organized, res, f), "")
		return env.ExitCode()
	}
	fmt.Fprintf(out, "\n%s\n\n", codesearch.FormatGrepHeader(pattern, res, limit))
	last := ""
	for i, h := range res.Hits {
		if i == 0 || h.Sub != last {
			if i > 0 {
				fmt.Fprintln(out)
			}
			fmt.Fprintln(out, codesearch.FormatGrepModule(h.Sub))
			last = h.Sub
		}
		fmt.Fprintln(out, codesearch.FormatGrepRow(h))
	}
	if res.Full(limit) {
		fmt.Fprintf(out, "\n  ... limit reached (%d) — use -n to increase\n", limit)
	}
	fmt.Fprintln(out)
	return env.ExitCode()
}
