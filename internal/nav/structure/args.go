// Package structure declares the module-nav structure commands (Types: hierarchy, implementors);
// they register into nav/registry from init.
package structure

import (
	"github.com/angeles725/niagara-go/internal/argparse"
	"github.com/angeles725/niagara-go/internal/nav/registry"
)

func init() {
	registry.Register("structure", []registry.Command{
		{Name: "hierarchy", Help: "Show subclass tree or inheritance chain", Args: argsHierarchy, Run: runHierarchy},
		{Name: "implementors", Help: "Show classes implementing an interface", Args: argsImplementors, Run: runImplementors},
	})
}

// argsHierarchy declares `hierarchy`.
func argsHierarchy(p *argparse.Parser) {
	p.Add(argparse.Arg{Help: "Class name (e.g. BEdgePane)"}, "name")
	p.Add(argparse.Arg{Default: 2, Type: argparse.Int, Help: "Max depth for subclass tree (default 2)"}, "--depth")
	p.Add(argparse.Arg{Action: argparse.StoreTrue, Help: "Show inheritance chain to root instead of subtree"}, "--chain")
}

// argsImplementors declares `implementors`.
func argsImplementors(p *argparse.Parser) {
	p.Add(argparse.Arg{Help: "Interface name (e.g. BIDialogPane)"}, "name")
	p.Add(argparse.Arg{Default: 50, Type: argparse.Int, Help: "Max results to show (default 50)"}, "-n")
}
