// Package modulefind is tools/module-find.py: a read-only finder over a Niagara N4
// module's Java source tree (slots, actions, writers, extends chains, ORD literals,
// grep, slot-type summary, the S19 ext-writable lint, schema compare, callers).
//
// Exit: 0 found, 1 a search command (slots, actions, ords, grep, callers, extends --of)
// matched nothing or writers found no static and no dynamic writer, 2 usage, 3 unreadable
// root or srcB, or no .java file.
package modulefind

import (
	"fmt"
	"io"
	"os"
	"slices"
	"sort"
	"strings"

	"github.com/angeles725/niagara-go/internal/argparse"
	"github.com/angeles725/niagara-go/internal/cli"
	"github.com/angeles725/niagara-go/internal/honest"
	"github.com/angeles725/niagara-go/internal/pycompat"
	"github.com/angeles725/niagara-go/internal/pyre"
	"github.com/angeles725/niagara-go/internal/version"
)

var (
	// searchCommands exit 1 when they match nothing (so do extends --of and writers, which
	// --limit does not cap).
	searchCommands = []string{"slots", "actions", "ords", "grep", "callers"}
	// limitCommands list rows, so --limit caps them; any other command rejects it.
	limitCommands = append(append([]string(nil), searchCommands...), "slot-types", "ext-writable", "compare")
)

// regexType is modulefind.sources.regex_type: a Python regex that compiles (IGNORECASE).
var regexType = &argparse.Type{Name: "regex_type", Convert: func(s string) (any, error) {
	re, err := pyre.Compile(s, pyre.IGNORECASE)
	if err != nil {
		return nil, argparse.NewTypeError("invalid regex %s: %s", pycompat.StrRepr(s), err)
	}
	return re, nil
}}

func buildParser() *argparse.Parser {
	top, common := argparse.OutputParent(false), argparse.OutputParent(true)
	p := argparse.New(argparse.Config{Prog: "modfind", Description: description,
		Formatter: argparse.RawDescriptionHelpFormatter, Parents: []*argparse.Parser{top}})
	p.Add(argparse.Arg{Action: argparse.Custom, Nargs: argparse.NoArgs, Default: argparse.Suppress,
		Dest: argparse.Suppress, Help: "print the tools version and exit", Call: func(c *argparse.Call) error {
			fmt.Fprintln(c.Out, version.Line("module-find"))
			return &argparse.ExitError{Code: 0}
		}}, "--version")
	p.Add(argparse.Arg{Nargs: argparse.Optional, Help: "module src root (omit for selftest)"}, "root")
	sub := p.AddSubparsers(argparse.SubparsersConfig{Dest: "cmd", Required: true})
	add := func(name, help string) *argparse.Parser {
		return sub.AddParser(name, argparse.SubparserConfig{Config: argparse.Config{Parents: []*argparse.Parser{common}},
			Help: help})
	}
	s := add("slots", "@NiagaraProperty declarations")
	s.Add(argparse.Arg{Help: "substring of the slot type"}, "--type")
	s.Add(argparse.Arg{Help: "require these flags (o/s/h/r/t or OPERATOR..)"}, "--flags")
	s.Add(argparse.Arg{Type: regexType, Help: "regex on slot name"}, "--name")
	add("actions", "@NiagaraAction declarations").Add(argparse.Arg{Type: regexType, Help: "regex on action name"}, "--name")
	add("writers", "who writes a slot").Add(argparse.Arg{}, "slot")
	add("extends", "class -> superclass map").Add(argparse.Arg{Help: "resolve the full chain of one class"}, "--of")
	add("ords", "ORD-shaped string literals").Add(argparse.Arg{Type: regexType, Help: "regex on the ORD"}, "--name")
	g := add("grep", "regex over class/slot/action names (--source: over the code)")
	g.Add(argparse.Arg{Type: regexType, Help: "Python regex, case-insensitive"}, "regex")
	g.Add(argparse.Arg{Action: argparse.StoreTrue, Help: "search every line of every .java file (file:line text)"}, "--source")
	add("slot-types", "per-type summary (count/OPERATOR/complex/TRANSIENT)").Add(
		argparse.Arg{Help: "(reserved) filter marker"}, "--module")
	add("ext-writable", "S19 lint preview: OPERATOR complex property without action")
	add("compare", "schema diff <root>(A,older) vs <srcB>(newer)").Add(
		argparse.Arg{Help: "source root B (newer) — <root> is A (older)"}, "srcB")
	add("callers", "who calls a method across the tree").Add(
		argparse.Arg{Help: "method name, e.g. setSetpoint / forceDefrost"}, "method")
	add("selftest", "in-memory assertions (no tree needed)")
	return p
}

// Main runs module-find with argv (without the program name).
func Main(args []string, s cli.Stdio) int {
	p := buildParser()
	ns, code, done := p.Parse(args, s)
	if done {
		return code
	}
	cmd := ns.String("cmd")
	if ns.Get("limit") != nil && !slices.Contains(limitCommands, cmd) {
		sorted := append([]string(nil), limitCommands...)
		sort.Strings(sorted)
		return p.Error(s.Err, fmt.Sprintf("--limit does not apply to `%s` (it caps the rows of: %s)", cmd,
			strings.Join(sorted, ", ")))
	}
	o := &opts{json: ns.Bool("json"), csv: ns.Bool("csv"), limit: ns.Int("limit"), out: s.Out, err: s.Err}
	if cmd == "selftest" {
		return selftest(s.Out)
	}
	root := ns.String("root")
	if root == "" {
		fmt.Fprintln(s.Err, "module-find: a src root is required for this command")
		return 2
	}
	m, code := openRoot(root, s.Err)
	if m == nil {
		return code
	}
	var b *module // compare's srcB, refused like <root> (exit 3) when it cannot be searched
	if cmd == "compare" {
		if b, code = openRoot(ns.String("srcB"), s.Err); b == nil {
			return code
		}
	}
	o.scope = scope(m, root)
	found := run(m, b, o, cmd, ns)
	paths := make([]string, 0, len(m.unreadable))
	for path := range m.unreadable {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	for _, path := range paths {
		fmt.Fprintf(s.Err, "module-find: unreadable, not searched: %s: %s\n", path, m.unreadable[path])
	}
	if found == 0 {
		return 1
	}
	return 0
}

// openRoot scans a module source root, or reports why it cannot be searched (exit 3): not a
// directory, or no .java file under it.
func openRoot(root string, stderr io.Writer) (*module, int) {
	if st, err := os.Stat(root); err != nil || !st.IsDir() {
		fmt.Fprintf(stderr, "module-find: not a directory: %s\n", root)
		return nil, 3
	}
	m := newModule(root)
	if m.files == 0 {
		return nil, honest.Fail(stderr, "module-find", fmt.Sprintf("no .java files under %s (dot-dirs pruned)", root), 3)
	}
	return m, 0
}

func regexArg(ns *argparse.Namespace, dest string) *pyre.Regexp {
	re, _ := ns.Get(dest).(*pyre.Regexp)
	return re
}

// run dispatches one command and returns what Main turns into the exit code. A command that
// can miss (a search, extends --of, writers) returns its match count, and Main exits 1 only on 0. Every
// other command (a listing or summary such as slot-types, ext-writable, extends, compare)
// returns -1, so it exits 0 even when it has nothing to list: an empty listing is an answer,
// not a miss. A new command returns -1 unless the exit contract says it can miss.
func run(m, b *module, o *opts, cmd string, ns *argparse.Namespace) int {
	switch cmd {
	case "slots":
		return cmdSlots(m, o, slotFilter{typ: ns.String("type"), hasType: ns.Get("type") != nil,
			flags: ns.String("flags"), hasFlag: ns.Get("flags") != nil, name: regexArg(ns, "name")})
	case "actions":
		return cmdActions(m, o, regexArg(ns, "name"))
	case "writers":
		return cmdWriters(m, o, ns.String("slot"))
	case "extends":
		return cmdExtends(m, o, ns.String("of"), ns.Get("of") != nil)
	case "ords":
		return cmdOrds(m, o, regexArg(ns, "name"))
	case "grep":
		return cmdGrep(m, o, regexArg(ns, "regex"), ns.Bool("source"))
	case "slot-types":
		cmdSlotTypes(m, o)
	case "ext-writable":
		cmdExtWritable(m, o)
	case "compare":
		cmdCompare(m, b, o)
	case "callers":
		return cmdCallers(m, o, ns.String("method"))
	}
	return -1
}
