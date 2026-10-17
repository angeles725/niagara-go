// Package lookup declares the module-nav lookup commands (Find classes and concepts);
// they register into nav/registry from init.
package lookup

import (
	"github.com/angeles725/niagara-go/internal/argparse"
	"github.com/angeles725/niagara-go/internal/nav/cliargs"
	"github.com/angeles725/niagara-go/internal/nav/registry"
)

func init() {
	registry.Register("lookup", []registry.Command{
		{Name: "class", Help: "Look up a class by exact name", Args: argsClass, Run: runClass},
		{Name: "search", Help: "Search classes by glob pattern", Args: argsSearch, Run: runSearch},
		{Name: "package", Help: "List classes in a package", Args: argsPackage, Run: runPackage},
		{Name: "profile", Help: "Combined class + source + UI view", Args: argsProfile},
		{Name: "full-profile", Help: "Combined view from BOTH navigators", Args: argsFullProfile},
		{Name: "cross-ref", Help: "Docs + implementation + who uses it", Args: argsCrossRef},
		{Name: "find", Help: "Fuzzy search in class names (camelCase-aware); see also: rank", Description: "Fuzzy search in class names (camelCase-aware). See also: `rank` for a ranked class lookup.", Args: argsFind},
		{Name: "ask", Help: "Same as `rank` (ranked classes for a question); see also: grep", Description: "Answers with `rank`: the classes most relevant to the question, best first. Use `grep` for literal text in the sources.", Args: argsAsk, Run: runAsk},
		{Name: "explain", Help: "Cross-reference a concept across devguide/guides/bajadoc + code; see also: rank, grep", Description: "Cross-reference a concept across devguide/guides/bajadoc + code. See also: `rank` for a ranked class lookup, `grep` for text in the sources.", Args: argsExplain},
		{Name: "unified", Help: "Search Help + Module + BOG simultaneously; see also: rank, grep", Description: "Search Help + Module + BOG simultaneously. See also: `rank` for a ranked class lookup, `grep` for text in the sources.", Args: argsUnified},
		{Name: "compare", Help: "Docs vs implementation vs station usage", Args: argsCompare},
	})
}

// argsClass declares `class`.
func argsClass(p *argparse.Parser) {
	p.Add(argparse.Arg{Help: "Class name (e.g. BLinkPad)"}, "name")
}

// argsSearch declares `search`.
func argsSearch(p *argparse.Parser) {
	p.Add(argparse.Arg{Help: "Glob pattern (e.g. '*Dialog*', 'BLink*')"}, "pattern")
	p.Add(argparse.Arg{Default: 50, Type: argparse.Int, Help: "Max results to show (default 50)"}, "-n")
}

// argsPackage declares `package`.
func argsPackage(p *argparse.Parser) {
	p.Add(argparse.Arg{Nargs: argparse.Optional, Help: "Package name (e.g. com.tridium.workbench.util)"}, "name")
	p.Add(argparse.Arg{Action: argparse.StoreTrue, Help: "List all packages with class counts", Dest: "show_all"}, "--all")
	p.Add(argparse.Arg{Default: 50, Type: argparse.Int, Help: "Max results to show (default 50)"}, "-n")
}

// argsProfile declares `profile`.
func argsProfile(p *argparse.Parser) {
	p.Add(argparse.Arg{Help: "Class name (e.g. BLinkPad)"}, "name")
}

// argsFullProfile declares `full-profile`.
func argsFullProfile(p *argparse.Parser) {
	p.Add(argparse.Arg{Help: "Class name (e.g. BAlarmService)"}, "name")
	p.Add(argparse.Arg{Help: "Help Navigator directory (auto-detected if omitted)"}, "--help-dir")
}

// argsCrossRef declares `cross-ref`.
func argsCrossRef(p *argparse.Parser) {
	p.Add(argparse.Arg{Help: "Class name (e.g. BWebServlet)"}, "name")
	p.Add(argparse.Arg{Help: "Help Navigator directory (auto-detected if omitted)"}, "--help-dir")
}

// argsFind declares `find`.
func argsFind(p *argparse.Parser) {
	p.Add(argparse.Arg{Nargs: argparse.OneOrMore, Help: "Search query words (e.g. 'alarm service')"}, "query")
	p.Add(argparse.Arg{Action: argparse.StoreTrue, Help: "Also search in source tokens (slower)"}, "--source")
	p.Add(argparse.Arg{Help: "Filter by module", Dest: "find_module"}, "--module")
	p.Add(argparse.Arg{Default: 50, Type: argparse.Int, Help: "Limit results (default 50)"}, "-n")
}

// argsAsk declares `ask`.
func argsAsk(p *argparse.Parser) {
	p.Add(argparse.Arg{Nargs: argparse.OneOrMore, Help: "Question in natural language (e.g. 'how does alarm routing work')"}, "question")
	p.Add(argparse.Arg{Default: 20, Type: cliargs.PositiveInt, Help: "Max classes to show (default 20)"}, "-n", "--limit")
}

// argsExplain declares `explain`.
func argsExplain(p *argparse.Parser) {
	p.Add(argparse.Arg{Help: "Concept to explain (e.g. 'SMA', 'skipModuleValidation')"}, "concept")
	p.Add(argparse.Arg{Help: "Override niagara-help root (default: $NIAGARA_HOME/niagara-help)", Dest: "explain_help_root"}, "--help-root")
	p.Add(argparse.Arg{Action: argparse.StoreTrue, Help: "Only do filename matching (skip content grep — faster)", Dest: "explain_no_content"}, "--no-content")
	p.Add(argparse.Arg{Default: 15, Type: argparse.Int, Help: "Max hits per doc tree (default 15)", Dest: "explain_n"}, "-n")
}

// argsUnified declares `unified`.
func argsUnified(p *argparse.Parser) {
	p.Add(argparse.Arg{Help: "Search query"}, "query")
	p.Add(argparse.Arg{Default: 10, Type: argparse.Int, Help: "Limit results per source (default 10)"}, "--limit")
}

// argsCompare declares `compare`.
func argsCompare(p *argparse.Parser) {
	p.Add(argparse.Arg{Help: "Class name to compare"}, "class_name")
}
