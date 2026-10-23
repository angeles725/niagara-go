// Package bognav is bog-nav: a read-only navigator for a Niagara N4 station config.bog (or a bare
// file.xml), the Go port of niagara-research tools/bog-nav.py and tools/bognav/.
//
// Files by command group, like the Python package: queries (tree, slot, find, types, grep, diff),
// links (links, handle, path), checks (writable, relays, hoa, tiles), pointcache, classify
// (writability, --src slot types), defaults (organized/ type defaults), output (rows, --json,
// --csv, --limit), selftest. Parsing is internal/bog; the declaration parser internal/slotomatic.
//
// Exit codes: 0 found, 1 searched and no match (find/links/grep/writable) or a named
// component/handle/slot not found, 2 usage error, 3 unreadable input (bog.ExitBadInput, D17).
package bognav

import (
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/angeles725/niagara-go/internal/argparse"
	"github.com/angeles725/niagara-go/internal/bog"
	"github.com/angeles725/niagara-go/internal/cli"
	"github.com/angeles725/niagara-go/internal/honest"
	"github.com/angeles725/niagara-go/internal/version"
)

// command runs one subcommand: found is the number of matching rows (search commands exit 1 on
// 0) and code an exit code, or -1 to let Main decide.
type command func(e *env) (found, code int)

var commands = map[string]command{
	"tree": cmdTree, "slot": cmdSlot, "links": cmdLinks, "handle": cmdHandle, "path": cmdPath,
	"find": cmdFind, "types": cmdTypes, "pointcache": cmdPointcache, "writable": cmdWritable,
	"relays": cmdRelays, "hoa": cmdHoa, "tiles": cmdTiles, "grep": cmdGrep, "diff": cmdDiff,
}

// searchCommands exit 1 when they match nothing (principle 2 of docs/TOOLING-GUIDELINES.md).
var searchCommands = map[string]bool{"find": true, "links": true, "grep": true, "writable": true}

// limitCommands are the row-listing commands --limit caps; any other command rejects it (exit 2).
var limitCommands = []string{"find", "types", "grep", "links", "writable", "relays", "hoa", "tiles", "pointcache"}

func newParser() *argparse.Parser {
	top, common := argparse.OutputParent(false), argparse.OutputParent(true)
	p := argparse.New(argparse.Config{Prog: "bognav", Description: description,
		Formatter: argparse.RawDescriptionHelpFormatter, Parents: []*argparse.Parser{top}})
	p.Add(argparse.Arg{Action: argparse.Custom, Nargs: argparse.NoArgs, Default: argparse.Suppress,
		Help: "print the tools version and exit", Call: func(c *argparse.Call) error {
			fmt.Fprintln(c.Out, version.Line("bog-nav"))
			return &argparse.ExitError{Code: 0}
		}}, "--version")
	p.Add(argparse.Arg{Nargs: argparse.Optional, Help: "config.bog (ZIP) or a bare file.xml (omit for selftest)"}, "bog")
	sub := p.AddSubparsers(argparse.SubparsersConfig{Dest: "cmd", Required: true})
	add := func(name, help string) *argparse.Parser {
		return sub.AddParser(name, argparse.SubparserConfig{Config: argparse.Config{Parents: []*argparse.Parser{common}}, Help: help})
	}
	type a = argparse.Arg
	flag := func(help string) a { return a{Action: argparse.StoreTrue, Help: help} }

	t := add("tree", "component tree")
	t.Add(a{Help: "filter by PFX:Type (or bare Type)"}, "--type")
	t.Add(a{Type: argparse.Int, Help: "max depth"}, "--depth")

	s := add("slot", "a component's slots + children")
	s.Add(a{Help: "path or h:handle"}, "ref")
	s.Add(a{Nargs: argparse.Optional, Help: "a specific slot"}, "slotname")
	s.Add(a{Help: "module src root: fill a bog-absent type from @NiagaraProperty"}, "--src")
	s.Add(a{Action: argparse.StoreTrue, Dest: "no_defaults",
		Help: "do not list the type defaults of the slots absent from the bog (organized/ catalog)"}, "--no-defaults")

	l := add("links", "links, sourceOrd resolved to paths")
	l.Add(a{Help: "target path/component"}, "--to")
	l.Add(a{Dest: "from_", Help: "source path/component"}, "--from")
	l.Add(a{Help: "exact slot name on the --from source end / the --to target end (either end when neither is given)"}, "--slot")
	l.Add(a{Action: argparse.StoreTrue, Dest: "slot_any", Help: "--slot matches either end regardless of --from/--to (pre-2026-09-06 behaviour)"}, "--slot-any")
	l.Add(flag("only links whose targetSlotName no @NiagaraProperty under --src declares (CHECK7; the JSON \"dangling\" field; not an unresolved source, see src_resolved)"), "--dangling")
	l.Add(a{Help: "module src root: resolve --dangling (CHECK7)"}, "--src")

	h := add("handle", "component at a handle + its links")
	h.Add(a{Help: "h:xxxx or xxxx"}, "handle")
	pa := add("path", "reverse: handle -> component path")
	pa.Add(a{Help: "h:xxxx or xxxx"}, "handle")

	fi := add("find", "flat list of components of a type")
	fi.Add(a{Required: true, Help: "PFX:Type (or bare Type)"}, "--type")
	fi.Add(flag("also match handle-less typed slots (value wrappers) the index alone misses"), "--slots")

	ty := add("types", "histogram of component types (characterise an unknown bog)")
	ty.Add(flag("also count handle-less typed slots (value wrappers etc.)"), "--slots")
	ty.Add(a{Dest: "type_filter", Help: "only types whose name contains this substring"}, "--grep")

	pc := add("pointcache", "Alerton Compass DisplayService PointCache: decode bacref ORDs + freshness")
	pc.Add(a{Help: "only this owning device instance"}, "--device")
	pc.Add(flag("only stale rows"), "--stale")
	pc.Add(flag("only fresh rows"), "--live")
	pc.Add(a{Dest: "stale_before", Help: "year YYYY; rows polled before it are STALE (default: newest year in the cache)"}, "--stale-before")
	pc.Add(flag("print only the freshness/year summary"), "--summary")

	w := add("writable", "classify direct value slots by external write shape")
	w.Add(a{Help: "filter by module prefix or name"}, "--module")
	w.Add(a{Choices: []any{"simple", "complex", "bare", "other"}, Help: "only this write class"}, "--klass")
	w.Add(a{Help: "module src root: fill a bog-absent type from @NiagaraProperty"}, "--src")

	r := add("relays", "CHECK11: own-module output -> writable proxy, fallback + writeOnUp")
	r.Add(a{Help: "filter by source module prefix or name"}, "--module")

	ho := add("hoa", "CHECK8: HOA/mode slots + values, flag non-auto leftovers")
	ho.Add(a{Help: "filter by module prefix or name"}, "--module")
	ho.Add(flag("also scan driver writables (priority arrays)"), "--all")

	add("tiles", "CHECK18: per-RoomPanel evapN tile -> unit wiring, flag crossed")

	g := add("grep", "regex over component paths, types, slot names")
	g.Add(a{}, "regex")

	d := add("diff", "delta vs another bog")
	d.Add(a{Help: "the other config.bog|file.xml"}, "bogB")

	add("selftest", "run in-memory parser/query assertions (no file needed)")
	return p
}

// Main runs bog-nav with args (without the program name).
func Main(args []string, s cli.Stdio) int {
	p := newParser()
	ns, code, done := p.Parse(args, s)
	if done {
		return code
	}
	cmd := ns.String("cmd")
	if ns.Get("limit") != nil && !slices.Contains(limitCommands, cmd) {
		sorted := append([]string(nil), limitCommands...)
		sort.Strings(sorted)
		return p.Error(s.Err, fmt.Sprintf("--limit does not apply to `%s` (it caps the rows of: %s)", cmd, strings.Join(sorted, ", ")))
	}
	if cmd == "tree" && ns.Get("depth") != nil && ns.Int("depth") < 0 {
		return p.Error(s.Err, fmt.Sprintf("--depth must be >= 0, got %d", ns.Int("depth")))
	}
	if cmd == "selftest" {
		return selftest(s.Out)
	}
	path := ns.String("bog")
	if path == "" {
		fmt.Fprintln(s.Err, "bog-nav: a config.bog|file.xml is required for this command")
		return cli.ExitCannotSearch
	}
	b, err := bog.Load(path)
	if err != nil {
		return honest.Fail(s.Err, "bog-nav", err.Error(), bog.ExitBadInput)
	}
	e := &env{ns: ns, out: s.Out, err: s.Err, bog: b, scope: scope(b)}
	found, code := commands[cmd](e)
	if e.failed {
		return cli.ExitCannotSearch
	}
	if code >= 0 {
		return code
	}
	if searchCommands[cmd] && found == 0 {
		return cli.ExitNoMatch // searched, no match: the miss line names the bog and its counts
	}
	return cli.ExitFound
}

// scope is Bog.scope: "searched <bog>: N components, M links".
func scope(b *bog.Bog) string {
	return honest.Scope(b.SrcPath, honest.Count{Label: "components", N: b.Comps.Len()}, honest.Count{Label: "links", N: len(b.Links)})
}
