package help

import "github.com/angeles725/niagara-go/internal/argparse"

// Report and coverage commands (cli/cmd_export.py): export, export-html,
// export-module, coverage, diff-api.
var exportCommands = []command{
	{name: "export", help: "Export full class report to markdown file",
		args: func(p *argparse.Parser) {
			positional(p, "name", "Class name (e.g. BAlarmService), or a module name with --module")
			positional(p, "file", "Output .md file path (.html with --html)")
			form := p.AddMutuallyExclusiveGroup(false)
			form.Add(argparse.Arg{Action: argparse.StoreTrue,
				Help: "Standalone HTML class report (replaces export-html)"}, "--html")
			form.Add(argparse.Arg{Action: argparse.StoreTrue,
				Help: "Full module report in markdown (replaces export-module)"}, "--module")
		}},
	{name: "export-html", help: "Export class report as standalone HTML file",
		aliasOf: "export --html",
		args: func(p *argparse.Parser) {
			positional(p, "name", "Class name (e.g. BAlarmService)")
			positional(p, "file", "Output .html file path")
		}},
	{name: "export-module", help: "Export full module report to markdown",
		aliasOf: "export --module",
		args: func(p *argparse.Parser) {
			positional(p, "name", "Module name (e.g. alarm-rt)")
			positional(p, "file", "Output .md file path")
		}},
	{name: "coverage", help: "Documentation coverage report (overview exits 1 on guide " +
		"folders with unindexed HTML pages)",
		args: func(p *argparse.Parser) {
			optionalPositional(p, "module", nil, "Module name (omit for overview)")
			flag(p, "Gate: every entry of every doc jar of this version must be "+
				"indexed or excluded with a reason (exit 1 on any gap)", "--doc-jars")
			flag(p, "Gate: every documentation-like entry of every jar of the "+
				"whole install (modules/, javadoc/, sw/, root) and every file "+
				"of its docs/ must be indexed or excluded by a rule of "+
				"install_docs.json (exit 1 on any unclassified entry)", "--sweep")
		}},
	{name: "diff-api", help: "Compare two API export JSON files",
		args: func(p *argparse.Parser) {
			positional(p, "file1", "First JSON file")
			positional(p, "file2", "Second JSON file")
		}},
}

// Maintenance and interactive commands (cli/cmd_admin.py): build-call-index,
// freshness, rebuild-all, selftest, repl.
var adminCommands = []command{
	{name: "build-call-index", help: "Build call-index.json (pre-computed method call graph)"},
	{name: "freshness", help: "Check whether indexes are in sync with the raw corpus",
		args: func(p *argparse.Parser) {
			p.Add(argparse.Arg{Dest: "fresh_all", Action: argparse.StoreTrue,
				Help: "One line per registered Niagara version (index + install drift)"}, "--all")
		},
		run: func(e *Env, ns *argparse.Namespace) error { return e.Freshness(ns.Bool("fresh_all")) }},
	{name: "rebuild-all", help: "Rebuild all indexes in dependency order, then stamp the manifest",
		args: func(p *argparse.Parser) {
			p.Add(argparse.Arg{Help: "Only run builders whose script starts with this"}, "--only")
			flag(p, "Print the build plan without executing", "--dry-run")
			flag(p, "Record current corpus as the baseline without rebuilding", "--stamp-only")
		}},
	{name: "selftest", help: "Assert core search behaviours (exits non-zero on failure)",
		args: func(p *argparse.Parser) {
			flag(p, "Recall oracle: every class/method/slot/java file/page in the "+
				"raw sources must be found by its lookup command", "--recall")
			flag(p, "With --recall: skip the sampled find/fuzzy rows", "--no-samples")
		}},
	{name: "repl", help: "Interactive mode (keeps indexes in memory)"},
}
