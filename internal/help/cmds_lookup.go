package help

import "github.com/angeles725/niagara-go/internal/argparse"

// Class lookup commands (cli/cmd_classes.py): class, search, hierarchy,
// package, implementors, enum, interface-hierarchy, archetypes, orphans.
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
	{name: "hierarchy", help: "Show subclass tree or inheritance chain",
		args: func(p *argparse.Parser) {
			positional(p, "name", "Class name (e.g. BComponent)")
			intOpt(p, nil, "Max depth for subclass tree (default: 2; 3 with --interface)", "--depth")
			flag(p, "Show inheritance chain upward", "--chain")
			flag(p, "Interface hierarchy: extends chain, sub-interfaces and "+
				"implementors (replaces interface-hierarchy)", "--interface")
		},
		run: func(e *Env, ns *argparse.Namespace) error {
			var depth *int
			if ns.Get("depth") != nil {
				d := ns.Int("depth")
				depth = &d
			}
			return e.Hierarchy(HierarchyParams{Name: ns.String("name"), Depth: depth,
				Chain: ns.Bool("chain"), Interface: ns.Bool("interface")})
		}},
	{name: "package", help: "Browse classes by package",
		args: func(p *argparse.Parser) {
			optionalPositional(p, "name", "", "Package name or partial (omit to list all)")
			flag(p, "List all packages", "--all")
		},
		run: func(e *Env, ns *argparse.Namespace) error {
			return e.Package(PackageParams{Name: ns.String("name"), All: ns.Bool("all")})
		}},
	{name: "implementors", help: "Find classes implementing an interface",
		args: func(p *argparse.Parser) {
			positional(p, "name", "Interface name (e.g. BIAlarmSource)")
		},
		run: func(e *Env, ns *argparse.Namespace) error {
			return e.Implementors(ImplementorsParams{Name: ns.String("name")})
		}},
	{name: "enum", help: "Show enum/frozen values and ordinals",
		args: func(p *argparse.Parser) {
			positional(p, "name", "Enum class name (e.g. BAckState)")
		}},
	{name: "interface-hierarchy", help: "Show interface inheritance hierarchy",
		aliasOf: "hierarchy --interface",
		args: func(p *argparse.Parser) {
			positional(p, "name", "Interface name (e.g. BIAlarmSource)")
			intOpt(p, 3, "Max depth for subtree (default: 3)", "--depth")
			flag(p, "Show extends chain upward", "--chain")
		},
		run: func(e *Env, ns *argparse.Namespace) error {
			return e.InterfaceHierarchy(InterfaceHierarchyParams{Name: ns.String("name"),
				Depth: ns.Int("depth"), Chain: ns.Bool("chain")})
		}},
	{name: "archetypes", help: "List Niagara architectural patterns (services, points, etc.)",
		args: func(p *argparse.Parser) {
			optionalPositional(p, "type", nil,
				"Archetype type (services, points, extensions, converters, etc.)")
		},
		run: func(e *Env, ns *argparse.Namespace) error {
			var typ *string
			if ns.Get("type") != nil {
				t := ns.String("type")
				typ = &t
			}
			return e.Archetypes(ArchetypesParams{Type: typ})
		}},
	{name: "orphans", help: "Find classes with zero used_by references",
		args: func(p *argparse.Parser) {
			intOpt(p, 30, "Max results to show (default: 30)", "-n", "--top")
		},
		run: func(e *Env, ns *argparse.Namespace) error {
			return e.Orphans(OrphansParams{Top: ns.Int("top")})
		}},
}

// Member and API commands (cli/cmd_members.py): method, method-sig, slots,
// inherited-slots, inherited-methods, api, overrides, contract, compatible.
var memberCommands = []command{
	{name: "method", help: "Search methods/fields/constructors by name",
		args: func(p *argparse.Parser) {
			positional(p, "name", "Method name or glob (e.g. 'doGet', 'set*')")
			p.Add(argparse.Arg{Choices: choices("methods", "fields", "constructors")}, "--section")
		},
		run: func(e *Env, ns *argparse.Namespace) error {
			return e.Method(MethodParams{Name: ns.String("name"), Section: ns.String("section")})
		}},
	{name: "method-sig", help: "Search methods by parameter/return type",
		args: func(p *argparse.Parser) {
			positional(p, "type", "Type to search (e.g. BAlarmRecord, WebOp)")
			p.Add(argparse.Arg{Choices: choices("param", "return", "any"), Default: "any",
				Help: "Search in params, return type, or both"}, "--direction")
			intOpt(p, 30, "", "-n", "--max-results")
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
	{name: "inherited-slots", help: "Show all slots including inherited from parents",
		aliasOf: "slots --inherited",
		args: func(p *argparse.Parser) {
			positional(p, "name", "Class name (e.g. BAlarmClass)")
			p.Add(argparse.Arg{Dest: "islots_json", Action: argparse.StoreTrue, Help: "Output as JSON"},
				"--json")
		},
		run: func(e *Env, ns *argparse.Namespace) error {
			return e.InheritedSlots(InheritedSlotsParams{Name: ns.String("name"), JSON: ns.Bool("islots_json")})
		}},
	{name: "inherited-methods", help: "Show all methods including inherited from parents",
		args: func(p *argparse.Parser) { positional(p, "name", "Class name") },
		run: func(e *Env, ns *argparse.Namespace) error {
			return e.InheritedMethods(InheritedMethodsParams{Name: ns.String("name")})
		}},
	{name: "api", help: "Public API surface (own methods + overrides, no framework noise)",
		args: func(p *argparse.Parser) {
			positional(p, "name", "Class name (e.g. BAlarmService)")
			flag(p, "Also show inherited methods from non-framework parents", "--inherited")
		},
		run: func(e *Env, ns *argparse.Namespace) error {
			return e.API(APIParams{Name: ns.String("name"), Inherited: ns.Bool("inherited")})
		}},
	{name: "overrides", help: "Show which methods a class overrides from parents",
		args: func(p *argparse.Parser) { positional(p, "name", "Class name") },
		run: func(e *Env, ns *argparse.Namespace) error {
			return e.Overrides(OverridesParams{Name: ns.String("name")})
		}},
	{name: "contract", help: "Show interface contract: methods + implementors",
		args: func(p *argparse.Parser) {
			positional(p, "name", "Interface name (e.g. BIAlarmSource)")
		},
		run: func(e *Env, ns *argparse.Namespace) error {
			return e.Contract(ContractParams{Name: ns.String("name")})
		}},
	{name: "compatible", help: "Find converters, points, and extensions for a type",
		args: func(p *argparse.Parser) {
			positional(p, "type", "Type name (e.g. BStatusNumeric)")
		},
		run: func(e *Env, ns *argparse.Namespace) error {
			return e.Compatible(CompatibleParams{Type: ns.String("type")})
		}},
}
