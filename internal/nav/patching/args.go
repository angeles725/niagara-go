// Package patching declares the module-nav patching commands (Patching);
// they register into nav/registry from init.
package patching

import (
	"github.com/angeles725/niagara-go/internal/argparse"
	"github.com/angeles725/niagara-go/internal/nav/registry"
)

func init() {
	registry.Register("patching", []registry.Command{
		{Name: "patch-target", Help: "Find patchable classes by natural language query", Args: argsPatchTarget},
		{Name: "patch-plan", Help: "Generate a complete patch plan for a class", Args: argsPatchPlan},
		{Name: "extract", Help: "Extract class to working directory for editing", Args: argsExtract},
		{Name: "export", Help: "Export class report or module diagram", Args: argsExport},
	})
}

// argsPatchTarget declares `patch-target`.
func argsPatchTarget(p *argparse.Parser) {
	p.Add(argparse.Arg{Help: "Natural language query (e.g. 'Link dialog height')"}, "query")
	p.Add(argparse.Arg{Default: 20, Type: argparse.Int, Help: "Max results to show (default 20)"}, "-n")
	p.Add(argparse.Arg{Help: "Working directory for patches (default: patches/)"}, "--patches-dir")
}

// argsPatchPlan declares `patch-plan`.
func argsPatchPlan(p *argparse.Parser) {
	p.Add(argparse.Arg{Help: "Class name (e.g. BLinkPad)"}, "name")
	p.Add(argparse.Arg{Help: "Working directory for patches (default: patches/)"}, "--patches-dir")
}

// argsExtract declares `extract`.
func argsExtract(p *argparse.Parser) {
	p.Add(argparse.Arg{Help: "Module name (e.g. workbench-wb)"}, "module")
	p.Add(argparse.Arg{Help: "Class name (e.g. BLinkPad)"}, "class_name")
	p.Add(argparse.Arg{Help: "Working directory for patches (default: patches/)"}, "--patches-dir")
}

// argsExport declares `export`.
func argsExport(p *argparse.Parser) {
	p.Add(argparse.Arg{Help: "Class name or module name"}, "name")
	p.Add(argparse.Arg{Nargs: argparse.Optional, Help: "Method name (for call-chain with --dot)"}, "method")
	p.Add(argparse.Arg{Action: argparse.StoreTrue, Help: "Generate standalone HTML class report"}, "--html")
	p.Add(argparse.Arg{Action: argparse.StoreTrue, Help: "Generate Mermaid class diagram for a module"}, "--mermaid")
	p.Add(argparse.Arg{Action: argparse.StoreTrue, Help: "Generate DOT graph (call-chain if method given, module if not)"}, "--dot")
	p.Add(argparse.Arg{Help: "Output file path (default: exports/<name>.html|.md|.dot)", Dest: "export_output"}, "-o", "--output")
}
