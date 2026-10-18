// Package flow declares the module-nav flow commands (Call and data flow);
// they register into nav/registry from init.
package flow

import (
	"github.com/angeles725/niagara-go/internal/argparse"
	"github.com/angeles725/niagara-go/internal/nav/registry"
)

func init() {
	registry.Register("flow", []registry.Command{
		{Name: "callers", Help: "Who calls this method (or, without one, every method of the class)", Args: argsCallers, Run: runCallers},
		{Name: "callees", Help: "What methods this class/method calls", Args: argsCallees, Run: runCallees},
	})
}

// argsCallers declares `callers`.
func argsCallers(p *argparse.Parser) {
	p.Add(argparse.Arg{Help: "Class name (e.g. BLinkPad)"}, "class_name")
	p.Add(argparse.Arg{Nargs: argparse.Optional, Help: "Method name (e.g. setPreferredSize); omit for every method of the class: per-method caller counts, then the call sites"}, "method")
	p.Add(argparse.Arg{Default: 50, Type: argparse.Int, Help: "Max results (default 50)"}, "-n")
}

// argsCallees declares `callees`.
func argsCallees(p *argparse.Parser) {
	p.Add(argparse.Arg{Help: "Class name (e.g. BLinkPad)"}, "class_name")
	p.Add(argparse.Arg{Nargs: argparse.Optional, Help: "Method name (optional; omit to see all class callees)"}, "method")
	p.Add(argparse.Arg{Default: 50, Type: argparse.Int, Help: "Max results (default 50)"}, "-n")
}
