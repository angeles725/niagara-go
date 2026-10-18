package searchdoor

import (
	"fmt"
	"strings"

	"github.com/angeles725/niagara-go/internal/argparse"
	"github.com/angeles725/niagara-go/internal/cli"
	"github.com/angeles725/niagara-go/internal/glossary"
	"github.com/angeles725/niagara-go/internal/pycompat"
	"github.com/angeles725/niagara-go/internal/version"
)

const exitUsage = 3

// Main is tools/nsearch.py.
func Main(args []string, s cli.Stdio) int { return run(args, s, "nsearch", newDoor(), defaultPaths) }

// Find3Main is tools/find3.py, the alias that differs only in its program name.
func Find3Main(args []string, s cli.Stdio) int { return run(args, s, "find3", newDoor(), defaultPaths) }

// valueOptions are the option strings of the common options that take a
// value: a `license` route word may follow any of them (`--limit 3 license x`).
var valueOptions = map[string]bool{"--limit": true, "-n": true, "--timeout": true, "--doc-version": true}

// splitLicense is door.split_license: (true, argv without the route word)
// when the first POSITIONAL word is `license` and a name follows it.
// Everything after `--` is query text.
func splitLicense(argv []string) (bool, []string) {
	var positions []int
	for i := 0; i < len(argv); {
		word := argv[i]
		if word == "--" {
			if len(positions) > 0 { // `license -- <name>`: the route word came first
				for j := i + 1; j < len(argv); j++ {
					positions = append(positions, j)
				}
			}
			break
		}
		if strings.HasPrefix(word, "-") && len([]rune(word)) > 1 {
			if valueOptions[word] {
				i += 2
			} else {
				i++
			}
			continue
		}
		positions = append(positions, i)
		i++
	}
	if len(positions) >= 2 && argv[positions[0]] == "license" {
		rest := append(append([]string(nil), argv[:positions[0]]...), argv[positions[0]+1:]...)
		return true, rest
	}
	return false, argv
}

func newParser(prog, description string) *argparse.Parser {
	return argparse.New(argparse.Config{Prog: prog, Description: description, NoAbbrev: true, ErrorExit: exitUsage})
}

// common is door._common: --version, --limit/-n, --timeout, --doc-version.
func common(p *argparse.Parser, tool string) {
	p.Add(argparse.Arg{Action: argparse.Custom, Nargs: argparse.NoArgs, Default: argparse.Suppress,
		Dest: argparse.Suppress, Help: "print the tools version and exit", Call: func(c *argparse.Call) error {
			fmt.Fprintln(c.Out, version.Line(tool))
			return &argparse.ExitError{Code: 0}
		}}, "--version")
	p.Add(argparse.Arg{Type: argparse.Int, Default: defaultLimit, Help: "ranked results per source (default 5)"},
		"--limit", "-n")
	p.Add(argparse.Arg{Type: argparse.Int, Default: 60,
		Help: "timeout budget per source in seconds, shared by all its backend calls (default 60)"}, "--timeout")
	p.Add(argparse.Arg{Metavar: "VERSION",
		Help: "niagara-help documentation version for the docs source (default: niagara-help's own)"}, "--doc-version")
}

func run(args []string, s cli.Stdio, prog string, d *door, paths func() Paths) int {
	isLicense, rest := splitLicense(args)
	var p *argparse.Parser
	if isLicense {
		p = newParser(prog+" license", "Licensing evidence for a module or a feature.")
		p.Add(argparse.Arg{Help: "module (bacnet-rt, baja) or license feature (bacnet, mstp)"}, "name")
		common(p, prog)
	} else {
		p = newParser(prog, "One query across our blocks, official docs and code.")
		p.Add(argparse.Arg{Nargs: argparse.OneOrMore, Help: "free-text query (`license <name>` for licensing)"}, "query")
		common(p, prog)
		g := p.AddMutuallyExclusiveGroup(false)
		g.Add(argparse.Arg{Dest: "mode", Action: argparse.StoreConst, Const: "and", Help: "every term (blocks, docs)"}, "--and")
		g.Add(argparse.Arg{Dest: "mode", Action: argparse.StoreConst, Const: "or", Help: "any term (blocks, docs)"}, "--or")
	}
	ns, code, done := p.Parse(rest, s)
	if done {
		return code
	}
	limit, timeout, docVersion := ns.Int("limit"), ns.Int("timeout"), ns.String("doc_version")
	if limit < 1 || timeout < 1 {
		return p.Error(s.Err, "--limit and --timeout must be at least 1")
	}
	src := paths()
	if ip, ok := d.backend.(*inprocBackend); ok {
		ip.paths = src
	}
	var outcomes []*Outcome
	hint := searchHint
	if isLicense {
		outcomes = d.license(s.Out, ns.String("name"), src, limit, timeout, docVersion)
		hint = licenseHint
	} else {
		query := pycompat.Strip(strings.Join(ns.Strings("query"), " "))
		if query == "" {
			fmt.Fprintf(s.Err, "%s: empty query\n", prog)
			return exitUsage
		}
		mode := ns.String("mode")
		header(s, query, mode, docVersion, src)
		outcomes = d.search(query, src, limit, mode, timeout, docVersion)
	}
	render(s.Out, outcomes, limit)
	return finish(s.Out, s.Err, outcomes, prog, hint)
}

// header prints the query line and how a Spanish question was read.
func header(s cli.Stdio, query, mode, docVersion string, p Paths) {
	modeText, docs := mode, docVersion
	if modeText == "" {
		modeText = "backend default"
	}
	if docs == "" {
		docs = "niagara-help default"
	}
	fmt.Fprintf(s.Out, "\nQuery: %s   mode=%s   docs=%s\n", pycompat.StrRepr(query), modeText, docs)
	read := glossary.Default().Note(query)
	// only the sources this front door expands are named: blocks (not in --and mode, where every added
	// word would become required) and the course code sub-scope (when present)
	var expanded []string
	if mode != "and" {
		expanded = append(expanded, "blocks")
	}
	if p.Course != "" {
		expanded = append(expanded, "course code")
	}
	switch {
	case read != "" && len(expanded) > 0:
		fmt.Fprintf(s.Out, "%s   (%s also search these English terms; the docs and the "+
			"module_nav code ranking apply their own glossary copies)\n", read, strings.Join(expanded, " and "))
	case read != "":
		fmt.Fprintf(s.Out, "%s   (--and searches the blocks as typed; the docs and the module_nav code ranking "+
			"apply their own glossary copies)\n", read)
	}
	fmt.Fprintln(s.Out)
}
