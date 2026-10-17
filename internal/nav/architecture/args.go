// Package architecture declares the module-nav architecture commands (Architecture and dependencies);
// they register into nav/registry from init.
package architecture

import (
	"github.com/angeles725/niagara-go/internal/argparse"
	"github.com/angeles725/niagara-go/internal/nav/registry"
)

func init() {
	registry.Register("architecture", []registry.Command{
		{Name: "patterns", Help: "Detect architectural patterns in a module", Args: argsPatterns},
		{Name: "pattern", Help: "List all classes matching a pattern category", Args: argsPattern},
		{Name: "throws", Help: "Methods with throws declarations in a class", Args: argsThrows},
		{Name: "catches", Help: "Classes that catch this exception type", Args: argsCatches},
		{Name: "cycles", Help: "Cyclic module dependencies (DFS/Tarjan)", Args: argsCycles},
		{Name: "god-classes", Help: "Classes with excessive methods/fields/deps", Args: argsGodClasses},
		{Name: "layer-check", Help: "Layer violations: RT must not import WB/UX", Args: argsLayerCheck},
		{Name: "complexity", Help: "Complexity metrics for a class", Args: argsComplexity},
		{Name: "complexity-scan", Help: "Ranking of most complex classes", Args: argsComplexityScan},
		{Name: "metrics", Help: "Module-level complexity summary", Args: argsMetrics},
	})
}

// argsPatterns declares `patterns`.
func argsPatterns(p *argparse.Parser) {
	p.Add(argparse.Arg{Help: "Module name (e.g. alarm-rt)"}, "name")
	p.Add(argparse.Arg{Default: 50, Type: argparse.Int, Help: "Max results per pattern (default 50)"}, "-n")
}

// argsPattern declares `pattern`.
func argsPattern(p *argparse.Parser) {
	p.Add(argparse.Arg{Help: "Pattern category (services, drivers, points, views, extensions, enums, structs)"}, "category")
	p.Add(argparse.Arg{Default: 50, Type: argparse.Int, Help: "Max results to show (default 50)"}, "-n")
	p.Add(argparse.Arg{Help: "Filter to a specific module", Dest: "pat_module"}, "--module", "-m")
}

// argsThrows declares `throws`.
func argsThrows(p *argparse.Parser) {
	p.Add(argparse.Arg{Help: "Class name (e.g. BAlarmService)"}, "name")
	p.Add(argparse.Arg{Default: 50, Type: argparse.Int, Help: "Max results to show (default 50)"}, "-n")
	p.Add(argparse.Arg{Help: "Filter to a specific module", Dest: "throws_module"}, "--module", "-m")
}

// argsCatches declares `catches`.
func argsCatches(p *argparse.Parser) {
	p.Add(argparse.Arg{Help: "Exception type (e.g. IOException)"}, "name")
	p.Add(argparse.Arg{Default: 50, Type: argparse.Int, Help: "Max results to show (default 50)"}, "-n")
	p.Add(argparse.Arg{Help: "Filter to a specific module", Dest: "catches_module"}, "--module", "-m")
}

// argsCycles declares `cycles`.
func argsCycles(p *argparse.Parser) {
	p.Add(argparse.Arg{Default: 6, Type: argparse.Int, Help: "Max cycle depth for DFS (default 6)"}, "--depth")
	p.Add(argparse.Arg{Default: 50, Type: argparse.Int, Help: "Max results to show (default 50)"}, "-n")
}

// argsGodClasses declares `god-classes`.
func argsGodClasses(p *argparse.Parser) {
	p.Add(argparse.Arg{Default: 100, Type: argparse.Int, Help: "Min score to qualify (default 100)"}, "--threshold", "-t")
	p.Add(argparse.Arg{Default: 50, Type: argparse.Int, Help: "Max results to show (default 50)"}, "-n")
}

// argsLayerCheck declares `layer-check`.
func argsLayerCheck(p *argparse.Parser) {
	p.Add(argparse.Arg{Nargs: argparse.Optional, Help: "Module to check (omit for full scan)"}, "layer_module")
}

// argsComplexity declares `complexity`.
func argsComplexity(p *argparse.Parser) {
	p.Add(argparse.Arg{Help: "Class name (e.g. BAlarmService)"}, "class_name")
}

// argsComplexityScan declares `complexity-scan`.
func argsComplexityScan(p *argparse.Parser) {
	p.Add(argparse.Arg{Help: "Filter by module", Dest: "cplx_module"}, "--module", "-m")
	p.Add(argparse.Arg{Default: 50, Type: argparse.Int, Help: "Max results to show (default 50)"}, "-n")
}

// argsMetrics declares `metrics`.
func argsMetrics(p *argparse.Parser) {
	p.Add(argparse.Arg{Help: "Module name (e.g. alarm-rt)"}, "metrics_module")
}
