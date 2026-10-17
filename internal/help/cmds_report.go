package help

import "github.com/angeles725/niagara-go/internal/argparse"

// Corpus report and maintenance commands the Go port keeps: stats
// (cli/cmd_analysis.py), freshness and selftest (cli/cmd_admin.py).
var adminCommands = []command{
	{name: "stats", help: "Corpus overview with counts and sizes",
		args: func(p *argparse.Parser) {
			p.Add(argparse.Arg{Dest: "stats_json", Action: argparse.StoreTrue, Help: "Output as JSON"},
				"--json")
		},
		run: func(e *Env, ns *argparse.Namespace) error { return e.Stats(ns.Bool("stats_json")) }},
	{name: "freshness", help: "Check whether indexes are in sync with the raw corpus",
		args: func(p *argparse.Parser) {
			p.Add(argparse.Arg{Dest: "fresh_all", Action: argparse.StoreTrue,
				Help: "One line per registered Niagara version (index + install drift)"}, "--all")
		},
		run: func(e *Env, ns *argparse.Namespace) error { return e.Freshness(ns.Bool("fresh_all")) }},
	{name: "selftest", help: "Assert core search behaviours (exits non-zero on failure)",
		args: func(p *argparse.Parser) {
			flag(p, "Recall oracle: every class/method/slot/java file/page in the "+
				"raw sources must be found by its lookup command", "--recall")
			flag(p, "With --recall: skip the sampled find/fuzzy rows", "--no-samples")
		},
		run: func(e *Env, ns *argparse.Namespace) error {
			return e.Selftest(SelftestParams{Recall: ns.Bool("recall"), NoSamples: ns.Bool("no_samples")})
		}},
}
