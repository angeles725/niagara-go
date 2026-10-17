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
		{Name: "grep", Help: "Regex search across all decompiled sources", Args: argsGrep},
		{Name: "rank", Help: "Classes most relevant to a free-text question (EN/ES), best first", Args: argsRank},
		{Name: "source", Help: "Show info or source code for a class", Args: argsSource},
		{Name: "snippet", Help: "Extract a specific method's source code", Args: argsSnippet},
		{Name: "imports", Help: "Raw import statements from the .java header (includes externals)", Args: argsImports},
		{Name: "strings", Help: "Search string literals across source; see also: grep", Description: "Search string literals across source. See also: `grep` for text in the sources.", Args: argsStrings},
		{Name: "resources", Help: "List non-Java resources in module JAR", Args: argsResources},
		{Name: "token", Help: "Exact token search (instant via SQLite index)", Args: argsToken},
		{Name: "resources-usage", Help: "How resources of a module are used", Args: argsResourcesUsage},
		{Name: "string-search", Help: "Regex search in indexed string literals (instant via SQLite); see also: grep", Description: "Regex search in indexed string literals (instant via SQLite). See also: `grep` for text in the sources.", Args: argsStringSearch},
		{Name: "string-constants", Help: "All string literals in a class (categorized)", Args: argsStringConstants},
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

// argsSnippet declares `snippet`.
func argsSnippet(p *argparse.Parser) {
	p.Add(argparse.Arg{Help: "Class name (e.g. BLinkPad)"}, "class_name")
	p.Add(argparse.Arg{Help: "Method name (e.g. openInDialog)"}, "method")
}

// argsImports declares `imports`.
func argsImports(p *argparse.Parser) {
	p.Add(argparse.Arg{Help: "Class name or FQN (e.g. BAlarmService or javax.baja.alarm.BAlarmService)"}, "name")
	p.Add(argparse.Arg{Action: argparse.StoreTrue, Help: "Only imports to classes NOT present in the decompiled corpus", Dest: "imports_external"}, "--external-only")
	p.Add(argparse.Arg{Default: 100, Type: argparse.Int, Help: "Max results to show (default 100)"}, "-n")
}

// argsStrings declares `strings`.
func argsStrings(p *argparse.Parser) {
	p.Add(argparse.Arg{Help: "Regex pattern to match in strings"}, "pattern")
	p.Add(argparse.Arg{Help: "Filter by module", Dest: "str_module"}, "--module", "-m")
	p.Add(argparse.Arg{Help: "Filter by class name", Dest: "str_class"}, "--class")
	p.Add(argparse.Arg{Default: 30, Type: argparse.Int, Help: "Max results (default 30)"}, "-n")
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

// argsResourcesUsage declares `resources-usage`.
func argsResourcesUsage(p *argparse.Parser) {
	p.Add(argparse.Arg{Help: "Module name (e.g. alarm-rt)"}, "name")
	p.Add(argparse.Arg{Default: 50, Type: argparse.Int, Help: "Limit results (default 50)"}, "-n")
}

// argsStringSearch declares `string-search`.
func argsStringSearch(p *argparse.Parser) {
	p.Add(argparse.Arg{Help: "Pattern to search (substring or regex)"}, "pattern")
	p.Add(argparse.Arg{Default: 50, Type: argparse.Int, Help: "Max results to show (default 50)"}, "-n")
	p.Add(argparse.Arg{Help: "Filter to a specific module", Dest: "ss_module"}, "--module", "-m")
}

// argsStringConstants declares `string-constants`.
func argsStringConstants(p *argparse.Parser) {
	p.Add(argparse.Arg{Help: "Class name (e.g. BAlarmService)"}, "class_name")
	p.Add(argparse.Arg{Default: 50, Type: argparse.Int, Help: "Max results to show (default 50)"}, "-n")
}
