// Package structure declares the module-nav structure commands (Types: hierarchy, members, imports);
// they register into nav/registry from init.
package structure

import (
	"github.com/angeles725/niagara-go/internal/argparse"
	"github.com/angeles725/niagara-go/internal/nav/registry"
)

func init() {
	registry.Register("structure", []registry.Command{
		{Name: "hierarchy", Help: "Show subclass tree or inheritance chain", Args: argsHierarchy},
		{Name: "implementors", Help: "Show classes implementing an interface", Args: argsImplementors},
		{Name: "xref", Help: "Show cross-references (imports/importers) for a class", Args: argsXref},
		{Name: "deps", Help: "Show module-level dependencies", Args: argsDeps},
		{Name: "method", Help: "Show classes that define a given method", Args: argsMethod},
		{Name: "methods", Help: "List all methods of a class", Args: argsMethods},
		{Name: "slots", Help: "Show Niagara slots (properties, actions, topics)", Args: argsSlots},
		{Name: "annotations", Help: "List classes with a specific Niagara annotation", Args: argsAnnotations},
		{Name: "deps-graph", Help: "Module dependency graph", Args: argsDepsGraph},
		{Name: "trace-type", Help: "Map Niagara type to Java class", Args: argsTraceType},
		{Name: "version-diff", Help: "Compare with another class-index.json", Args: argsVersionDiff},
		{Name: "fields", Help: "List all fields of a class", Args: argsFields},
		{Name: "field", Help: "Show classes that define a given field", Args: argsField},
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

// argsXref declares `xref`.
func argsXref(p *argparse.Parser) {
	p.Add(argparse.Arg{Help: "Class name (e.g. BLinkPad)"}, "name")
	p.Add(argparse.Arg{Action: argparse.StoreTrue, Help: "Show only classes that import this class"}, "--importers")
	p.Add(argparse.Arg{Action: argparse.StoreTrue, Help: "Show only what this class imports"}, "--imports")
	p.Add(argparse.Arg{Default: 50, Type: argparse.Int, Help: "Max results to show (default 50)"}, "-n")
}

// argsDeps declares `deps`.
func argsDeps(p *argparse.Parser) {
	p.Add(argparse.Arg{Help: "Module name (e.g. workbench-wb)"}, "name")
	p.Add(argparse.Arg{Action: argparse.StoreTrue, Help: "Show modules that depend on this one"}, "--reverse")
	p.Add(argparse.Arg{Default: 50, Type: argparse.Int, Help: "Max results to show (default 50)"}, "-n")
}

// argsMethod declares `method`.
func argsMethod(p *argparse.Parser) {
	p.Add(argparse.Arg{Help: "Method name (e.g. setPreferredSize)"}, "name")
	p.Add(argparse.Arg{Default: 50, Type: argparse.Int, Help: "Max results to show (default 50)"}, "-n")
	p.Add(argparse.Arg{Help: "Filter to a specific module", Dest: "method_module"}, "--module", "-m")
	p.Add(argparse.Arg{Help: "Show full signature in that class", Dest: "method_class"}, "--class")
}

// argsMethods declares `methods`.
func argsMethods(p *argparse.Parser) {
	p.Add(argparse.Arg{Help: "Class name (e.g. BLinkPad)"}, "name")
	p.Add(argparse.Arg{Default: 50, Type: argparse.Int, Help: "Max results to show (default 50)"}, "-n")
	p.Add(argparse.Arg{Action: argparse.StoreTrue, Help: "Show only public methods", Dest: "methods_public"}, "--public")
	p.Add(argparse.Arg{Help: "Filter methods by regex pattern", Dest: "methods_grep"}, "--grep")
}

// argsSlots declares `slots`.
func argsSlots(p *argparse.Parser) {
	p.Add(argparse.Arg{Nargs: argparse.Optional, Help: "Class name (e.g. BAlarmRecord)"}, "name")
	p.Add(argparse.Arg{Action: argparse.StoreTrue, Help: "Show only properties"}, "--properties")
	p.Add(argparse.Arg{Action: argparse.StoreTrue, Help: "Show only actions"}, "--actions")
	p.Add(argparse.Arg{Action: argparse.StoreTrue, Help: "Show only topics"}, "--topics")
	p.Add(argparse.Arg{Help: "Find classes with properties of this type (e.g. BStatusNumeric)"}, "--by-type")
	p.Add(argparse.Arg{Help: "Filter by module", Dest: "slots_module"}, "--module", "-m")
	p.Add(argparse.Arg{Default: 50, Type: argparse.Int, Help: "Max results to show (default 50)"}, "-n")
}

// argsAnnotations declares `annotations`.
func argsAnnotations(p *argparse.Parser) {
	p.Add(argparse.Arg{Help: "Annotation name (e.g. NiagaraType, NiagaraAction)"}, "name")
	p.Add(argparse.Arg{Default: 50, Type: argparse.Int, Help: "Max results to show (default 50)"}, "-n")
	p.Add(argparse.Arg{Help: "Filter by module", Dest: "ann_module"}, "--module", "-m")
}

// argsDepsGraph declares `deps-graph`.
func argsDepsGraph(p *argparse.Parser) {
	p.Add(argparse.Arg{Help: "Module name (e.g. workbench-wb)"}, "name")
	p.Add(argparse.Arg{Default: 1, Type: argparse.Int, Help: "Traversal depth (default 1)"}, "--depth")
	p.Add(argparse.Arg{Default: "mermaid", Choices: []any{"mermaid", "dot", "text"}, Help: "Output format (default: mermaid)", Dest: "graph_format"}, "--format")
	p.Add(argparse.Arg{Action: argparse.StoreTrue, Help: "Show dependents instead of dependencies"}, "--reverse")
}

// argsTraceType declares `trace-type`.
func argsTraceType(p *argparse.Parser) {
	p.Add(argparse.Arg{Help: "Type spec (e.g. 'control:NumericWritable' or 'BAlarmService')"}, "type_spec")
}

// argsVersionDiff declares `version-diff`.
func argsVersionDiff(p *argparse.Parser) {
	p.Add(argparse.Arg{Help: "Path to another class-index.json"}, "other_index")
	p.Add(argparse.Arg{Default: 50, Type: argparse.Int, Help: "Max results per section (default 50)"}, "-n")
}

// argsFields declares `fields`.
func argsFields(p *argparse.Parser) {
	p.Add(argparse.Arg{Help: "Class name (e.g. BAlarmService)"}, "name")
	p.Add(argparse.Arg{Default: 50, Type: argparse.Int, Help: "Max results to show (default 50)"}, "-n")
	p.Add(argparse.Arg{Action: argparse.StoreTrue, Help: "Show only static fields/constants", Dest: "fields_static"}, "--static")
	p.Add(argparse.Arg{Action: argparse.StoreTrue, Help: "Show only public fields", Dest: "fields_public"}, "--public")
	p.Add(argparse.Arg{Help: "Filter fields by regex pattern", Dest: "fields_grep"}, "--grep")
}

// argsField declares `field`.
func argsField(p *argparse.Parser) {
	p.Add(argparse.Arg{Help: "Field name (e.g. alarmDb)"}, "name")
	p.Add(argparse.Arg{Default: 50, Type: argparse.Int, Help: "Max results to show (default 50)"}, "-n")
	p.Add(argparse.Arg{Help: "Filter to a specific module", Dest: "field_module"}, "--module", "-m")
}
