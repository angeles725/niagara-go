package help

import "github.com/angeles725/niagara-go/internal/argparse"

// Cross-reference and module analysis commands (cli/cmd_analysis.py): module,
// xref, related, stats, module-deps, compare, profile, graph, hotspots,
// module-summary.
var analysisCommands = []command{
	{name: "module", help: "Cross-reference summary: classes + guides + PDFs",
		args: func(p *argparse.Parser) { positional(p, "name", "Module name (e.g. 'alarm')") },
		run:  func(e *Env, ns *argparse.Namespace) error { return e.Module(ns.String("name")) }},
	{name: "xref", help: "Show import cross-references",
		args: func(p *argparse.Parser) {
			positional(p, "name", "Class name")
			p.Add(argparse.Arg{Choices: choices("uses", "used_by", "both"), Default: "both"},
				"--direction")
		},
		run: func(e *Env, ns *argparse.Namespace) error {
			return e.Xref(XrefParams{Name: ns.String("name"), Direction: ns.String("direction")})
		}},
	{name: "related", help: "Discovery: related classes by package, hierarchy, xref, guides",
		args: func(p *argparse.Parser) { positional(p, "name", "Class name") },
		run:  func(e *Env, ns *argparse.Namespace) error { return e.Related(ns.String("name")) }},
	{name: "stats", help: "Corpus overview with counts and sizes",
		args: func(p *argparse.Parser) {
			p.Add(argparse.Arg{Dest: "stats_json", Action: argparse.StoreTrue, Help: "Output as JSON"},
				"--json")
		},
		run: func(e *Env, ns *argparse.Namespace) error { return e.Stats(ns.Bool("stats_json")) }},
	{name: "module-deps", help: "Show module dependency graph",
		args: func(p *argparse.Parser) {
			positional(p, "name", "Module name (e.g. alarm-rt)")
			flag(p, "Show modules that depend on this one", "--reverse")
		},
		run: func(e *Env, ns *argparse.Namespace) error {
			return e.ModuleDeps(ModuleDepsParams{Name: ns.String("name"), Reverse: ns.Bool("reverse")})
		}},
	{name: "compare", help: "Side-by-side comparison of two classes",
		args: func(p *argparse.Parser) {
			positional(p, "class1", "First class name")
			positional(p, "class2", "Second class name")
		}},
	{name: "profile", help: "Full class profile: identity, hierarchy, slots, methods, xref",
		args: func(p *argparse.Parser) { positional(p, "name", "Class name (e.g. BAlarmService)") },
		run:  func(e *Env, ns *argparse.Namespace) error { return e.Profile(ns.String("name")) }},
	{name: "graph", help: "ASCII dependency graph for a class",
		args: func(p *argparse.Parser) {
			positional(p, "name", "Class name")
			intOpt(p, 15, "", "-n", "--max-nodes")
		},
		run: func(e *Env, ns *argparse.Namespace) error {
			return e.Graph(GraphParams{Name: ns.String("name"), MaxNodes: ns.Int("max_nodes")})
		}},
	{name: "hotspots", help: "Most referenced/complex classes and most used types",
		args: func(p *argparse.Parser) {
			intOpt(p, 20, "Number of top entries to show (default: 20)", "-n", "--top")
		},
		run: func(e *Env, ns *argparse.Namespace) error { return e.Hotspots(ns.Int("top")) }},
	{name: "module-summary", help: "Full module overview: classes, deps, slots, API summary",
		args: func(p *argparse.Parser) { positional(p, "name", "Module name (e.g. alarm-rt)") },
		run:  func(e *Env, ns *argparse.Namespace) error { return e.ModuleSummary(ns.String("name")) }},
}

// Development-guidance commands (cli/cmd_develop.py): recipe, checklist,
// topics, action-params, event-flow.
var developCommands = []command{
	{name: "recipe", help: "Development recipe for a concept (alarm, service, driver, etc.)",
		args: func(p *argparse.Parser) {
			positional(p, "query", "Concept to look up (e.g. 'alarm extension')")
		}},
	{name: "checklist", help: "Implementation checklist for a base class",
		args: func(p *argparse.Parser) {
			positional(p, "base_class", "Base class (e.g. BAbstractService)")
		}},
	{name: "topics", help: "Show topics fired by a class and potential subscribers",
		args: func(p *argparse.Parser) { positional(p, "name", "Class name (e.g. BAlarmService)") },
		run:  func(e *Env, ns *argparse.Namespace) error { return e.Profile(ns.String("name")) }},
	{name: "action-params", help: "Find actions that accept a given parameter type",
		args: func(p *argparse.Parser) {
			positional(p, "type", "Parameter type (e.g. BAlarmRecord)")
			intOpt(p, 30, "", "-n", "--max-results")
		}},
	{name: "event-flow", help: "Trace event flow for a topic name",
		args: func(p *argparse.Parser) {
			positional(p, "topic", "Topic name or partial match")
			intOpt(p, 20, "", "-n", "--max-results")
		}},
}
