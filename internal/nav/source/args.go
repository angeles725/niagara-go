// Package source declares the module-nav source commands (Source text, strings and resources);
// they register into nav/registry from init.
package source

import (
	"github.com/angeles725/niagara-go/internal/argparse"
	"github.com/angeles725/niagara-go/internal/nav/cliargs"
	"github.com/angeles725/niagara-go/internal/nav/registry"
)

func init() {
	registry.Register("source", []registry.Command{
		{Name: "grep", Help: "Regex search across all decompiled sources", Args: argsGrep, Run: runGrep},
		{Name: "rank", Help: "Classes most relevant to a free-text question (EN/ES), best first", Args: argsRank, Run: runRank},
		{Name: "source", Help: "Show info or source code for a class", Args: argsSource, Run: runSource},
		{Name: "resources", Help: "List non-Java resources in module JAR", Args: argsResources, Run: runResources},
		{Name: "token", Help: "Exact token search (instant via SQLite index)", Args: argsToken, Run: runToken},
	})
}

// argsGrep declares `grep`.
func argsGrep(p *argparse.Parser) {
	p.Add(argparse.Arg{Help: "Regex pattern (e.g. 'setPreferredSize')"}, "pattern")
	p.Add(argparse.Arg{Default: 30, Type: argparse.Int, Help: "Max results to show (default 30)"}, "-n")
	p.Add(argparse.Arg{Help: "Filter to a specific module (e.g. workbench-wb)", Dest: "grep_module"}, "--module", "-m")
	p.Add(argparse.Arg{Help: "Filter by submodule type: rt, wb, ux, doc, se, standalone (<module>/<module>) or other (unknown suffix)", Dest: "grep_type"}, "--type", "-t")
}

// argsRank declares `rank`.
func argsRank(p *argparse.Parser) {
	p.Add(argparse.Arg{Nargs: argparse.OneOrMore, Help: "Question or keywords (e.g. 'como funciona el ack de alarmas', BWebServlet)"}, "question")
	p.Add(argparse.Arg{Default: 20, Type: cliargs.PositiveInt, Help: "Max classes to show (default 20)"}, "-n", "--limit")
}

// argsSource declares `source`.
func argsSource(p *argparse.Parser) {
	p.Add(argparse.Arg{Nargs: argparse.Optional, Help: "Class name (e.g. BLinkPad). Omit when using --batch."}, "name")
	p.Add(argparse.Arg{Action: argparse.StoreTrue, Help: "Show full source code with line numbers"}, "--code")
	p.Add(argparse.Arg{Help: "Search regex within the class source", Dest: "src_grep"}, "--grep")
	p.Add(argparse.Arg{Help: "CSV of class names to read in sequence (e.g. BLinkPad,BAlarmService)", Dest: "src_batch"}, "--batch")
}

// argsResources declares `resources`.
func argsResources(p *argparse.Parser) {
	p.Add(argparse.Arg{Help: "Module name (e.g. workbench-wb)"}, "name")
	p.Add(argparse.Arg{Help: "Filter by file extension (e.g. xml, properties)", Dest: "res_type"}, "--type")
	p.Add(argparse.Arg{Default: 100, Type: argparse.Int, Help: "Max results (default 100)"}, "-n")
}

// argsToken declares `token`.
func argsToken(p *argparse.Parser) {
	p.Add(argparse.Arg{Help: "Token to search (e.g. setPreferredSize)"}, "word")
	p.Add(argparse.Arg{Default: 50, Type: argparse.Int, Help: "Max results to show (default 50)"}, "-n")
	p.Add(argparse.Arg{Nargs: argparse.Optional, Const: 2, Default: 0, Type: argparse.Int, Help: "Show N surrounding source lines per match (bare flag = 2, omitted = 0 compact, N = custom window)", Metavar: "N", Dest: "token_context"}, "--context")
	p.Add(argparse.Arg{Help: "Filter to a specific module", Dest: "token_module"}, "--module", "-m")
}
