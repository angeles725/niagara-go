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

// Documentation search commands (cli/cmd_docs.py): guide, guide-search,
// devguide, devguide-search, pdf, find, fuzzy.
var docCommands = []command{
	{name: "guide", help: "Search guides by topic/folder",
		args: func(p *argparse.Parser) { positional(p, "query", "Search term (e.g. 'bacnet')") }},
	{name: "guide-search", help: "Full-text search in guides-clean/ with relevance ranking",
		args: func(p *argparse.Parser) {
			positional(p, "query", "Search text (e.g. 'discovery timeout')")
			intOpt(p, 10, "", "-n", "--max-results")
			andOr(p, "gs_mode")
		}},
	{name: "devguide", help: "Search Developer Guide TOC",
		args: func(p *argparse.Parser) { positional(p, "query", "Search term (e.g. 'component model')") }},
	{name: "devguide-search", help: "Full-text search in devguide-clean/ with relevance ranking",
		args: func(p *argparse.Parser) {
			positional(p, "query", "Search text")
			intOpt(p, 10, "", "-n", "--max-results")
			andOr(p, "ds_mode")
		}},
	{name: "pdf", help: "Search PDFs by name and full-text",
		args: func(p *argparse.Parser) {
			positional(p, "query", "Search term")
			flag(p, "", "--no-fulltext")
			p.Add(argparse.Arg{Choices: choices("software", "hardware")}, "--category")
		}},
	{name: "find", help: "Unified full-text search across ALL sources with relevance ranking",
		args: func(p *argparse.Parser) {
			positional(p, "query", "Search text")
			intOpt(p, 5, "", "-n", "--max-per-source")
			andOr(p, "find_mode")
		}},
	{name: "fuzzy", help: "Approximate search across classes, methods, slots, guides",
		args: func(p *argparse.Parser) {
			positional(p, "query", "Search query (multiple words OK)")
			intOpt(p, 15, "Number of top results per category (default: 15)", "-n", "--top")
		}},
}

// Java source commands (cli/cmd_source.py): source, source-grep, snippet,
// callers, chain, examples, annotations.
var sourceCommands = []command{
	{name: "source", help: "Look up or view Java source code",
		args: func(p *argparse.Parser) {
			positional(p, "name", "Class name or glob")
			flag(p, "Show full source code", "--code")
			p.Add(argparse.Arg{Dest: "grep_pattern", Help: "Grep pattern in source file"}, "--grep")
		}},
	{name: "source-grep", help: "Full-text regex search across ALL source files",
		args: func(p *argparse.Parser) {
			positional(p, "pattern", "Regex pattern (e.g. 'getSession\\(\\)')")
			intOpt(p, 30, "", "-n", "--max-results")
		}},
	{name: "snippet", help: "Extract a single method's source code",
		args: func(p *argparse.Parser) {
			positional(p, "class_name", "Class name")
			positional(p, "method", "Method name")
		}},
	{name: "callers", help: "Find classes that call a specific method",
		args: func(p *argparse.Parser) {
			positional(p, "method", "Method name (e.g. doRouteAlarm)")
			intOpt(p, 30, "", "-n", "--max-results")
		}},
	{name: "chain", help: "Multi-hop call chain tracing for a method",
		args: func(p *argparse.Parser) {
			positional(p, "method", "Method name (e.g. doRouteAlarm)")
			intOpt(p, 3, "Max recursion depth (default: 3)", "--depth")
			intOpt(p, 10, "Max callers per level (default: 10)", "--max-per-level")
		}},
	{name: "examples", help: "Find usage examples of a class across source files",
		args: func(p *argparse.Parser) {
			positional(p, "name", "Class name (e.g. BWebServlet)")
			intOpt(p, 15, "", "-n", "--max-results")
		}},
	{name: "annotations", help: "Search source files for Niagara annotations",
		args: func(p *argparse.Parser) {
			positional(p, "annotation", "Annotation name (e.g. NiagaraAction, @NiagaraProperty)")
			p.Add(argparse.Arg{Help: "Filter by type= value inside annotation"}, "--type-filter")
			intOpt(p, 30, "", "-n", "--max-results")
		}},
}
