package help

import "github.com/angeles725/niagara-go/internal/argparse"

// Class lookup commands (cli/cmd_classes.py) the Go port keeps: class,
// search, package. The rest are retired (docs/retired-commands.md).
var classCommands = []command{
	{name: "class", help: "Look up a class by name (package, inheritance, methods)",
		args: func(p *argparse.Parser) {
			positional(p, "name", "Class name (e.g. BWebServlet)")
			flag(p, "", "-v", "--verbose")
			flag(p, "Look up the JDK API (separate opt-in index) instead of Niagara", "--jdk")
		},
		run: func(e *Env, ns *argparse.Namespace) error {
			return e.Class(ClassParams{Name: ns.String("name"), Verbose: ns.Bool("verbose"), JDK: ns.Bool("jdk")})
		}},
	{name: "search", help: "Search classes by glob pattern",
		args: func(p *argparse.Parser) {
			positional(p, "pattern", "Glob pattern (e.g. 'BAlarm*')")
			flag(p, "Search the JDK API index instead of Niagara classes", "--jdk")
		},
		run: func(e *Env, ns *argparse.Namespace) error {
			return e.Search(SearchParams{Pattern: ns.String("pattern"), JDK: ns.Bool("jdk")})
		}},
	{name: "package", help: "Browse classes by package",
		args: func(p *argparse.Parser) {
			optionalPositional(p, "name", "", "Package name or partial (omit to list all)")
			flag(p, "List all packages", "--all")
		},
		run: func(e *Env, ns *argparse.Namespace) error {
			return e.Package(PackageParams{Name: ns.String("name"), All: ns.Bool("all")})
		}},
}

// Member commands (cli/cmd_members.py) the Go port keeps: method, slots.
var memberCommands = []command{
	{name: "method", help: "Search methods/fields/constructors by name",
		args: func(p *argparse.Parser) {
			positional(p, "name", "Method name or glob (e.g. 'doGet', 'set*')")
			p.Add(argparse.Arg{Choices: choices("methods", "fields", "constructors")}, "--section")
		},
		run: func(e *Env, ns *argparse.Namespace) error {
			return e.Method(MethodParams{Name: ns.String("name"), Section: ns.String("section")})
		}},
	{name: "slots", help: "Search/view @NiagaraProperty, @NiagaraAction, @NiagaraTopic",
		args: func(p *argparse.Parser) {
			optionalPositional(p, "query", "", "Class name or slot name/pattern")
			p.Add(argparse.Arg{Dest: "slot_type", Choices: choices("properties", "actions", "topics")},
				"--type")
			p.Add(argparse.Arg{Dest: "by_type", Help: "Search by property TYPE (e.g. BStatusNumeric)"},
				"--by-type")
			p.Add(argparse.Arg{Dest: "slots_json", Action: argparse.StoreTrue, Help: "Output as JSON"},
				"--json")
			flag(p, "All slots of the class QUERY including those inherited from "+
				"its parents (replaces inherited-slots; no --type / --by-type)", "--inherited")
		},
		run: func(e *Env, ns *argparse.Namespace) error {
			return e.Slots(SlotsParams{Query: ns.String("query"), SlotType: ns.String("slot_type"),
				ByType: ns.String("by_type"), JSON: ns.Bool("slots_json"), Inherited: ns.Bool("inherited")})
		}},
}

// moduleDetails are the list flags of `module` and their help.
var moduleDetails = [][2]string{
	{"types", "List the types the module registers (module:Type and class)"},
	{"agents", "List the agents (views, editors) the module's types register and the types they are on"},
	{"permissions", "List the java-permissions by profile and the module-permissions.xml templates the jar ships"},
	{"defs", "List the defs (name = value)"},
	{"schemas", "List the .xsd/.dtd schemas the jar ships (root element, targetNamespace)"},
}

// Module descriptor commands (Go-only, modules.go, palettes.go and types.go):
// module and type over indexes/module-index.json, palette over
// indexes/palette-index.json.
var moduleCommands = []command{
	{name: "module", help: "Module descriptor (module.xml): dependencies both ways, types, permissions",
		args: func(p *argparse.Parser) {
			positional(p, "name", "Module name, part name or glob (e.g. kitControl-rt, kitControl, 'kit*')")
			list := p.AddMutuallyExclusiveGroup(false)
			for _, d := range moduleDetails {
				list.Add(argparse.Arg{Dest: "module_" + d[0], Action: argparse.StoreTrue, Help: d[1]}, "--"+d[0])
			}
			p.Add(argparse.Arg{Dest: "module_json", Action: argparse.StoreTrue, Help: "Output as JSON"}, "--json")
		},
		run: func(e *Env, ns *argparse.Namespace) error {
			detail := ""
			for _, d := range moduleDetails {
				if ns.Bool("module_" + d[0]) {
					detail = d[0]
				}
			}
			return e.Module(ModuleParams{Name: ns.String("name"), Detail: detail, JSON: ns.Bool("module_json")})
		}},
	{name: "palette", help: "Shipped palettes (*.palette): where a type or folder sits, or a module's palette tree",
		args: func(p *argparse.Parser) {
			positional(p, "query", "Type, module:Type, entry name, folder or glob; with --module, a module name")
			p.Add(argparse.Arg{Dest: "palette_module", Action: argparse.StoreTrue,
				Help: "Print the palette trees of the module named by query (e.g. kitControl-rt)"}, "--module")
			p.Add(argparse.Arg{Dest: "palette_json", Action: argparse.StoreTrue, Help: "Output as JSON"}, "--json")
		},
		run: func(e *Env, ns *argparse.Namespace) error {
			return e.Palette(PaletteParams{Query: ns.String("query"), Module: ns.Bool("palette_module"), JSON: ns.Bool("palette_json")})
		}},
	{name: "type", help: "Niagara type (module:Type): its module, class, docs and the agents (views) on it",
		args: func(p *argparse.Parser) {
			positional(p, "name", "module:Type or Type, or a glob (e.g. kitControl:BooleanLatch, BooleanLatch)")
			p.Add(argparse.Arg{Dest: "type_json", Action: argparse.StoreTrue, Help: "Output as JSON"}, "--json")
		},
		run: func(e *Env, ns *argparse.Namespace) error {
			return e.Type(TypeParams{Name: ns.String("name"), JSON: ns.Bool("type_json")})
		}},
}
