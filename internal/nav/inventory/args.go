// Package inventory declares the module-nav inventory commands (Inventory and self-checks);
// they register into nav/registry from init.
package inventory

import (
	"github.com/angeles725/niagara-go/internal/argparse"
	"github.com/angeles725/niagara-go/internal/nav/cliargs"
	"github.com/angeles725/niagara-go/internal/nav/registry"
)

func init() {
	registry.Register("inventory", []registry.Command{
		{Name: "selftest", Help: "Self-checks; --recall verifies `class` finds every declaration in organized/", Args: argsSelftest, Run: runSelftest},
		{Name: "inventory", Help: "Summary of all indexed modules", Run: runInventory},
		{Name: "module", Help: "Detailed info for a specific submodule", Args: argsModule, Run: runModule},
		{Name: "modules", Help: "List modules with filters", Args: argsModules, Run: runModules},
		{Name: "stats", Help: "Corpus summary (modules, classes, indexes)", Args: argsStats, Run: runStats},
	})
}

// argsSelftest declares `selftest`.
func argsSelftest(p *argparse.Parser) {
	p.Add(argparse.Arg{Action: argparse.StoreTrue, Help: "Run the class-lookup recall oracle"}, "--recall")
	p.Add(argparse.Arg{Type: cliargs.PositiveInt, Help: "Check only N randomly chosen files (fast)"}, "--sample")
}

// argsModule declares `module`.
func argsModule(p *argparse.Parser) {
	p.Add(argparse.Arg{Help: "Submodule name (e.g. workbench-wb)"}, "name")
	p.Add(argparse.Arg{Action: argparse.StoreTrue, Help: "Show only the <permissions> block from module.xml", Dest: "mod_show_perms"}, "--permissions")
	p.Add(argparse.Arg{Action: argparse.StoreTrue, Help: "Show only the <dependencies> block from module.xml", Dest: "mod_show_deps"}, "--dependencies")
	p.Add(argparse.Arg{Action: argparse.StoreTrue, Help: "Cross-reference the module with installed license features (SMA-exempt, expiration, feature attributes)", Dest: "mod_show_sma"}, "--sma")
}

// argsModules declares `modules`.
func argsModules(p *argparse.Parser) {
	p.Add(argparse.Arg{Help: "Filter by type: rt, wb, ux, doc, se, standalone", Dest: "mod_type"}, "--type", "-t")
	p.Add(argparse.Arg{Action: argparse.StoreTrue, Help: "Show only ZKM-obfuscated modules"}, "--zkm")
	p.Add(argparse.Arg{Action: argparse.StoreTrue, Help: "Show only modules without Java code"}, "--no-code")
	p.Add(argparse.Arg{Action: argparse.StoreTrue, Help: "Show only modules with Java code"}, "--has-code")
	p.Add(argparse.Arg{Type: argparse.Int, Help: "Filter by bytecode version (e.g. 52 for Java 8)"}, "--bytecode", "-b")
	p.Add(argparse.Arg{Type: argparse.Int, Help: "Show top N modules by java_files count"}, "--top")
}

// argsStats declares `stats`.
func argsStats(p *argparse.Parser) {
	p.Add(argparse.Arg{Action: argparse.StoreTrue, Help: "Output in JSON format"}, "--json")
}
