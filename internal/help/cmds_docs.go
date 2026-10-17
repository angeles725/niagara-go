package help

import "github.com/angeles725/niagara-go/internal/argparse"

// andOr adds the --and (default) / --or mutually exclusive pair: whether every
// query term must appear (cmd_docs.py _and_or).
func andOr(p *argparse.Parser, dest string) {
	mode := p.AddMutuallyExclusiveGroup(false)
	mode.Add(argparse.Arg{Dest: dest + "_and", Action: argparse.StoreTrue, Default: false,
		Help: "ALL terms must appear in each file (default)"}, "--and")
	mode.Add(argparse.Arg{Dest: dest + "_or", Action: argparse.StoreTrue, Default: false,
		Help: "ANY term in file"}, "--or")
}

// Documentation search commands (cli/cmd_docs.py) the Go port keeps: guide,
// guide-search, devguide, devguide-search, pdf, find.
var docCommands = []command{
	{name: "guide", help: "Search guides by topic/folder",
		args: func(p *argparse.Parser) { positional(p, "query", "Search term (e.g. 'bacnet')") },
		run:  func(e *Env, ns *argparse.Namespace) error { return e.Guide(ns.String("query")) }},
	{name: "guide-search", help: "Full-text search in guides-clean/ with relevance ranking",
		args: func(p *argparse.Parser) {
			positional(p, "query", "Search text (e.g. 'discovery timeout')")
			intOpt(p, 10, "", "-n", "--max-results")
			andOr(p, "gs_mode")
		},
		run: func(e *Env, ns *argparse.Namespace) error {
			return e.GuideSearch(FulltextParams{Query: ns.String("query"), Max: ns.Int("max_results"), Or: ns.Bool("gs_mode_or")})
		}},
	{name: "devguide", help: "Search Developer Guide TOC",
		args: func(p *argparse.Parser) { positional(p, "query", "Search term (e.g. 'component model')") },
		run:  func(e *Env, ns *argparse.Namespace) error { return e.Devguide(ns.String("query")) }},
	{name: "devguide-search", help: "Full-text search in devguide-clean/ with relevance ranking",
		args: func(p *argparse.Parser) {
			positional(p, "query", "Search text")
			intOpt(p, 10, "", "-n", "--max-results")
			andOr(p, "ds_mode")
		},
		run: func(e *Env, ns *argparse.Namespace) error {
			return e.DevguideSearch(FulltextParams{Query: ns.String("query"), Max: ns.Int("max_results"), Or: ns.Bool("ds_mode_or")})
		}},
	{name: "pdf", help: "Search PDFs by name and full-text",
		args: func(p *argparse.Parser) {
			positional(p, "query", "Search term")
			flag(p, "", "--no-fulltext")
			p.Add(argparse.Arg{Choices: choices("software", "hardware")}, "--category")
		},
		run: func(e *Env, ns *argparse.Namespace) error {
			var category *string
			if ns.Get("category") != nil {
				c := ns.String("category")
				category = &c
			}
			return e.PDF(PDFParams{Query: ns.String("query"), NoFulltext: ns.Bool("no_fulltext"), Category: category})
		}},
	{name: "find", help: "Unified full-text search across ALL sources with relevance ranking",
		args: func(p *argparse.Parser) {
			positional(p, "query", "Search text")
			intOpt(p, 5, "", "-n", "--max-per-source")
			andOr(p, "find_mode")
		},
		run: func(e *Env, ns *argparse.Namespace) error {
			return e.Find(FulltextParams{Query: ns.String("query"), Max: ns.Int("max_per_source"), Or: ns.Bool("find_mode_or")})
		}},
}

// Java source commands (cli/cmd_source.py) the Go port keeps: source,
// source-grep.
var sourceCommands = []command{
	{name: "source", help: "Look up or view Java source code",
		args: func(p *argparse.Parser) {
			positional(p, "name", "Class name or glob")
			flag(p, "Show full source code", "--code")
			p.Add(argparse.Arg{Dest: "grep_pattern", Help: "Grep pattern in source file"}, "--grep")
		},
		run: func(e *Env, ns *argparse.Namespace) error {
			return e.Source(SourceParams{Name: ns.String("name"), Code: ns.Bool("code"),
				Grep: ns.String("grep_pattern")})
		}},
	{name: "source-grep", help: "Full-text regex search across ALL source files",
		args: func(p *argparse.Parser) {
			positional(p, "pattern", "Regex pattern (e.g. 'getSession\\(\\)')")
			intOpt(p, 30, "", "-n", "--max-results")
		}},
}
