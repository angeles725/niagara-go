// Package corpusnav is the cnav command (port of tools/corpus-nav.py): navigation
// of the niagara-research knowledge corpus (blocks, sub-corpora, docs, retros) —
// find, grep, show, list, by-marker, by-focus, connections, graph, path, lint-refs,
// stats, semantic (TF-IDF; the embedding backend is retired, decision D10) and selftest.
//
// The corpus root is $CORPUS_NAV_ROOT, else the niagara-research checkout found by
// walking up from the executable and then the working directory (locate.ResearchCheckout),
// else the parent of the directory holding the executable; $CORPUS_NAV_INDEX_DIR moves the find cache (default <root>/tools/.corpus-index)
// and $CORPUS_NAV_FIND_INDEX=off ranks find in memory. A root without block files is not
// the corpus: every lookup there exits 2 (cannot search) instead of reporting no match.
package corpusnav

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/angeles725/niagara-go/internal/argparse"
	"github.com/angeles725/niagara-go/internal/blockfiles"
	"github.com/angeles725/niagara-go/internal/cli"
	"github.com/angeles725/niagara-go/internal/locate"
	"github.com/angeles725/niagara-go/internal/version"
)

// defaultRoot locates the corpus when $CORPUS_NAV_ROOT is unset: the first
// niagara-research checkout (locate.ResearchCheckout) walking up from the
// executable, then from the working directory; outside any checkout, the
// executable's grandparent, which the cannot-search message then names. Tests
// replace it.
var defaultRoot = func() (string, error) {
	if root, ok := locate.Find(locate.Options{Marker: locate.ResearchCheckout,
		Starts: []locate.Start{locate.FromExecutable, locate.FromCwd}}); ok {
		return root, nil
	}
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	if exe, err = filepath.EvalSymlinks(exe); err != nil {
		return "", err
	}
	return filepath.Dir(filepath.Dir(exe)), nil
}

func buildParser() *argparse.Parser {
	p := argparse.New(argparse.Config{Prog: "cnav", Formatter: argparse.RawDescriptionHelpFormatter,
		Description: "Navigate the niagara-research knowledge corpus (blocks, docs, retros)."})
	p.Add(argparse.Arg{Action: argparse.Custom, Nargs: argparse.NoArgs, Default: argparse.Suppress,
		Dest: argparse.Suppress, Help: "print the tools version and exit", Call: func(c *argparse.Call) error {
			fmt.Fprintln(c.Out, version.Line("corpus-nav"))
			return &argparse.ExitError{Code: 0}
		}}, "--version")
	sub := p.AddSubparsers(argparse.SubparsersConfig{Dest: "command", Metavar: "<command>"})
	add := func(name, help string, aliases ...string) *argparse.Parser {
		return sub.AddParser(name, argparse.SubparserConfig{Help: help, Aliases: aliases})
	}
	scope := func(q *argparse.Parser) {
		q.Add(argparse.Arg{Dest: "scope", Choices: []any{"blocks", "docs", "retros", "all"}, Default: "blocks",
			Help: "corpus subset to search (default: blocks)"}, "--in")
		q.Add(argparse.Arg{Type: argparse.PositiveInt, Default: 50, Help: "max matching lines to print (default: 50)"},
			"--limit", "-n")
	}
	bare := func(q *argparse.Parser) {
		q.Add(argparse.Arg{Action: argparse.StoreTrue,
			Help: "also count bare B<N> mentions (outside code) of existing blocks as citations"}, "--bare")
	}
	find := add("find", "full-text case-insensitive tokenized search with ranking")
	find.Add(argparse.Arg{}, "query")
	scope(find)
	mode := find.AddMutuallyExclusiveGroup(false)
	mode.Add(argparse.Arg{Dest: "mode_and", Action: argparse.StoreTrue, Help: "ALL terms must appear in a block (AND mode)"}, "--and")
	mode.Add(argparse.Arg{Dest: "mode_or", Action: argparse.StoreTrue, Help: "ANY term matches (OR mode, default)"}, "--or")
	grep := add("grep", "regex (re module) search")
	grep.Add(argparse.Arg{}, "regex")
	scope(grep)
	add("show", "H1 + outline + Connections of block B<N>").Add(argparse.Arg{Type: argparse.Int, Metavar: "N"}, "n")
	add("list", "list all blocks (from CATALOG.md)").Add(argparse.Arg{Help: "filter to blocks belonging to a focus"}, "--focus")
	add("by-marker", "blocks containing an evidence marker").Add(argparse.Arg{Help: "e.g. INFER, CERT, CERT-hw, CERT-live"}, "marker")
	add("by-focus", "blocks belonging to a focus").Add(argparse.Arg{}, "focus")
	conn := add("connections", "forward + reverse block links for B<N>")
	conn.Add(argparse.Arg{Metavar: "N", Help: "block id: 720, B720, 1-3 or TI"}, "n")
	bare(conn)
	g := add("graph", "whole citation graph (summary, or --json adjacency)")
	g.Add(argparse.Arg{Action: argparse.StoreTrue, Help: "emit {nodes, forward, reverse, dangling, stats} as JSON"}, "--json")
	g.Add(argparse.Arg{Dest: "top", Type: argparse.PositiveInt, Default: 10,
		Help: "most-cited blocks to list in the summary (default: 10; --top is the old name)"}, "--limit", "--top", "-n")
	bare(g)
	path := add("path", "shortest citation chain between two blocks")
	path.Add(argparse.Arg{Metavar: "A"}, "a")
	path.Add(argparse.Arg{Metavar: "B"}, "b")
	path.Add(argparse.Arg{Action: argparse.StoreTrue, Help: "follow forward citations only (default: either direction)"}, "--directed")
	bare(path)
	lint := add("lint-refs", "flag non-canonical citations (exit 1 if any)")
	lint.Add(argparse.Arg{Nargs: argparse.ZeroOrMore, Metavar: "FILE", Help: "files to lint (default: every block)"}, "files")
	lint.Add(argparse.Arg{Action: argparse.StoreTrue, Help: "report only; always exit 0"}, "--warn")
	lint.Add(argparse.Arg{Type: argparse.PositiveInt, Default: 50, Help: "max findings to print (default: 50; the summary counts all)"},
		"--limit", "-n")
	add("stats", "corpus totals + marker counts")
	add("selftest", "assert core search behaviours (exits non-zero on failure)")
	// The embedding options of the Python tool stay accepted so existing scripts keep
	// working (decision D10: semantic ranks with TF-IDF only).
	sem := add("semantic", "rank blocks by TF-IDF similarity to a query", "rank")
	sem.Add(argparse.Arg{Nargs: argparse.Optional, Help: "natural-language query string"}, "query")
	sem.Add(argparse.Arg{Type: argparse.PositiveInt, Default: 15, Help: "number of top results to show (default: 15)"}, "--limit", "-n")
	sem.Add(argparse.Arg{Action: argparse.StoreTrue,
		Help: "retired: there is no embedding index; exits 2 (kept for compatibility)"}, "--build-index")
	sem.Add(argparse.Arg{Help: "ignored; kept for compatibility (embeddings are retired)"}, "--model")
	sem.Add(argparse.Arg{Action: argparse.StoreTrue,
		Help: "kept for compatibility; the ranking is always TF-IDF"}, "--tfidf")
	sem.Add(argparse.Arg{Type: argparse.Int, Help: "ignored; kept for compatibility (embeddings are retired)"}, "--threads")
	return p
}

// Main runs corpus-nav with argv (without the program name).
func Main(args []string, s cli.Stdio) int {
	p := buildParser()
	ns, code, done := p.Parse(args, s)
	if done {
		return code
	}
	cmd := ns.String("command")
	if cmd == "" {
		p.PrintHelp(s.Out)
		return 0
	}
	root := os.Getenv("CORPUS_NAV_ROOT")
	if root == "" {
		r, err := defaultRoot()
		if err != nil {
			fmt.Fprintf(s.Err, "corpus-nav: cannot locate the corpus: %v\n", err)
			return 2
		}
		root = r
	}
	if readsCorpus(cmd, ns) && len(blockfiles.MainBlocks(root)) == 0 {
		fmt.Fprintf(s.Err, "corpus-nav: cannot search: no block files under %s "+
			"(set CORPUS_NAV_ROOT to the niagara-research checkout)\n", pyPath(root))
		return 2
	}
	c := newCorpus(root, os.Getenv("CORPUS_NAV_INDEX_DIR"), s.Out, s.Err)
	str := ns.String
	switch cmd {
	case "find":
		return c.cmdFind(str("query"), str("scope"), ns.Int("limit"), ns.Bool("mode_and"))
	case "grep":
		return c.cmdGrep(str("regex"), str("scope"), ns.Int("limit"))
	case "show":
		return c.cmdShow(ns.Int("n"))
	case "list":
		return c.cmdList(str("focus"))
	case "by-marker":
		return c.cmdByMarker(str("marker"))
	case "by-focus":
		return c.cmdByFocus(str("focus"))
	case "connections":
		return c.cmdConnections(str("n"), ns.Bool("bare"))
	case "graph":
		return c.cmdGraph(ns.Bool("json"), ns.Int("top"), ns.Bool("bare"))
	case "path":
		return c.cmdPath(str("a"), str("b"), ns.Bool("directed"), ns.Bool("bare"))
	case "lint-refs":
		return c.cmdLintRefs(ns.Strings("files"), ns.Bool("warn"), ns.Int("limit"))
	case "stats":
		return c.cmdStats()
	case "selftest":
		return c.cmdSelftest()
	case "semantic", "rank":
		return c.cmdSemantic(str("query"), ns.Get("query") != nil, ns.Bool("build_index"), ns.Bool("tfidf"), ns.Int("limit"))
	}
	fmt.Fprintf(s.Err, "corpus-nav: unknown command %s\n", strings.TrimSpace(cmd))
	return 2
}

// readsCorpus says whether cmd looks anything up in the corpus. lint-refs FILE... reads
// only the files it is given, and semantic reports its own input errors first.
func readsCorpus(cmd string, ns *argparse.Namespace) bool {
	switch cmd {
	case "lint-refs":
		return len(ns.Strings("files")) == 0
	case "semantic", "rank":
		return !ns.Bool("build_index") && ns.Get("query") != nil
	}
	return true
}
