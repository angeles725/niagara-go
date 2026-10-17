// Package flow declares the module-nav flow commands (Call and data flow);
// they register into nav/registry from init.
package flow

import (
	"github.com/angeles725/niagara-go/internal/argparse"
	"github.com/angeles725/niagara-go/internal/nav/registry"
)

func init() {
	registry.Register("flow", []registry.Command{
		{Name: "callers", Help: "Who calls this method (or, without one, every method of the class)", Args: argsCallers},
		{Name: "callees", Help: "What methods this class/method calls", Args: argsCallees},
		{Name: "call-chain", Help: "Transitive call chain (tree)", Args: argsCallChain},
		{Name: "hotspots", Help: "Most-called methods (hub analysis)", Args: argsHotspots},
		{Name: "type-consumers", Help: "Methods that receive this type as parameter", Args: argsTypeConsumers},
		{Name: "type-producers", Help: "Methods that return this type", Args: argsTypeProducers},
		{Name: "type-flow", Help: "Producer -> consumer chain for a type", Args: argsTypeFlow},
		{Name: "module-calls", Help: "Methods of target module called from source module", Args: argsModuleCalls},
		{Name: "module-api-usage", Help: "Top external methods used by a module", Args: argsModuleApiUsage},
		{Name: "coupling", Help: "Coupling metrics between two modules", Args: argsCoupling},
		{Name: "impact", Help: "Transitive impact analysis for a class or method", Args: argsImpact},
		{Name: "virtual-callers", Help: "Aggregate callers via virtual dispatch (inheritance chain)", Args: argsVirtualCallers},
		{Name: "full-stack-trace", Help: "Call chain with semantic annotations (input, BQL, data-source, serialize, output)", Args: argsFullStackTrace},
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

// argsCallChain declares `call-chain`.
func argsCallChain(p *argparse.Parser) {
	p.Add(argparse.Arg{Help: "Class name (e.g. BAlarmService)"}, "class_name")
	p.Add(argparse.Arg{Help: "Method name (e.g. routeAlarm)"}, "method")
	p.Add(argparse.Arg{Default: 2, Type: argparse.Int, Help: "Max tree depth (default 2)"}, "--depth")
}

// argsHotspots declares `hotspots`.
func argsHotspots(p *argparse.Parser) {
	p.Add(argparse.Arg{Default: 20, Type: argparse.Int, Help: "Max results (default 20)"}, "-n")
	p.Add(argparse.Arg{Help: "Filter by module", Dest: "hot_module"}, "--module", "-m")
}

// argsTypeConsumers declares `type-consumers`.
func argsTypeConsumers(p *argparse.Parser) {
	p.Add(argparse.Arg{Help: "Type name (e.g. BAlarmRecord)"}, "type_name")
	p.Add(argparse.Arg{Default: 50, Type: argparse.Int, Help: "Max results to show (default 50)"}, "-n")
	p.Add(argparse.Arg{Help: "Filter to a specific module", Dest: "tc_module"}, "--module", "-m")
}

// argsTypeProducers declares `type-producers`.
func argsTypeProducers(p *argparse.Parser) {
	p.Add(argparse.Arg{Help: "Type name (e.g. BOrd)"}, "type_name")
	p.Add(argparse.Arg{Default: 50, Type: argparse.Int, Help: "Max results to show (default 50)"}, "-n")
	p.Add(argparse.Arg{Help: "Filter to a specific module", Dest: "tp_module"}, "--module", "-m")
}

// argsTypeFlow declares `type-flow`.
func argsTypeFlow(p *argparse.Parser) {
	p.Add(argparse.Arg{Help: "Type name (e.g. BAlarmRecord)"}, "type_name")
	p.Add(argparse.Arg{Default: 1, Type: argparse.Int, Help: "Trace depth via call graph (default 1)"}, "--depth")
	p.Add(argparse.Arg{Default: 30, Type: argparse.Int, Help: "Max results per section (default 30)"}, "-n")
}

// argsModuleCalls declares `module-calls`.
func argsModuleCalls(p *argparse.Parser) {
	p.Add(argparse.Arg{Help: "Source module (e.g. alarm-rt)"}, "from_mod")
	p.Add(argparse.Arg{Help: "Target module (e.g. baja)"}, "to_mod")
	p.Add(argparse.Arg{Default: 50, Type: argparse.Int, Help: "Max results to show (default 50)"}, "-n")
}

// argsModuleApiUsage declares `module-api-usage`.
func argsModuleApiUsage(p *argparse.Parser) {
	p.Add(argparse.Arg{Help: "Module name (e.g. workbench-wb)"}, "name")
	p.Add(argparse.Arg{Default: 50, Type: argparse.Int, Help: "Max results to show (default 50)"}, "-n")
}

// argsCoupling declares `coupling`.
func argsCoupling(p *argparse.Parser) {
	p.Add(argparse.Arg{Help: "First module (e.g. alarm-rt)"}, "mod1")
	p.Add(argparse.Arg{Help: "Second module (e.g. baja)"}, "mod2")
}

// argsImpact declares `impact`.
func argsImpact(p *argparse.Parser) {
	p.Add(argparse.Arg{Help: "Class name (e.g. BLinkPad)"}, "class_name")
	p.Add(argparse.Arg{Nargs: argparse.Optional, Help: "Method name (optional; omit for class-level impact)"}, "method")
	p.Add(argparse.Arg{Default: 3, Type: argparse.Int, Help: "Max transitive depth (default 3)"}, "--depth")
}

// argsVirtualCallers declares `virtual-callers`.
func argsVirtualCallers(p *argparse.Parser) {
	p.Add(argparse.Arg{Help: "Base class (e.g. BControlPoint)"}, "class_name")
	p.Add(argparse.Arg{Help: "Method name (e.g. execute)"}, "method_name")
	p.Add(argparse.Arg{Default: 0, Type: argparse.Int, Help: "Max subclass depth (0=unlimited, default 0)", Dest: "vc_depth"}, "--depth")
	p.Add(argparse.Arg{Action: argparse.StoreTrue, Help: "Exclude direct callers of the base class", Dest: "vc_no_self"}, "--no-self")
	p.Add(argparse.Arg{Action: argparse.StoreTrue, Help: "Structured JSON output", Dest: "vc_json"}, "--json")
}

// argsFullStackTrace declares `full-stack-trace`.
func argsFullStackTrace(p *argparse.Parser) {
	p.Add(argparse.Arg{Help: "Class.method to trace (e.g. BAlarmServlet.doGet)"}, "class_method")
	p.Add(argparse.Arg{Default: 6, Type: argparse.Int, Help: "Max traversal depth (default 6)"}, "--depth")
	p.Add(argparse.Arg{Default: "all", Choices: []any{"all", "data"}, Help: "'all' = show all annotations; 'data' = only data-interesting nodes (default: all)"}, "--annotate")
	p.Add(argparse.Arg{Action: argparse.StoreTrue, Help: "Output trace as JSON instead of plain text", Dest: "fst_json"}, "--json")
}
