package help

import (
	"math"
	"math/big"
	"sort"
	"strings"

	"github.com/angeles725/niagara-go/internal/argparse"
)

// command is one subcommand (cli/registry.py Command): its name, its help
// line, the canonical form it is an alias of, the function declaring its
// arguments and the function running it (nil: not ported yet).
type command struct {
	name    string
	help    string
	aliasOf string
	args    func(p *argparse.Parser)
	run     func(e *Env, ns *argparse.Namespace) error
}

// helpText is the line listed in --help; an alias says which form replaces
// it (registry.py command): "Alias of 'X': " plus the help, first letter
// lower-cased.
func (c *command) helpText() string {
	if c.aliasOf == "" || c.help == "" {
		return c.help
	}
	r := []rune(c.help)
	return "Alias of '" + c.aliasOf + "': " + strings.ToLower(string(r[0])) + string(r[1:])
}

// commandGroups are the cmd_*.py modules, in their import order, holding
// the commands the Go port keeps.
var commandGroups = [][]command{
	classCommands, memberCommands, docCommands, sourceCommands, adminCommands,
	versionCommands, maintCommands, moduleCommands,
}

// registry maps a command name to its definition (registry.py COMMANDS).
var registry = buildRegistry()

func buildRegistry() map[string]*command {
	r := map[string]*command{}
	for _, group := range commandGroups {
		for i := range group {
			c := &group[i]
			if _, dup := r[c.name]; dup {
				panic("help: command registered twice: " + c.name)
			}
			r[c.name] = c
		}
	}
	return r
}

// order is the order of the subcommands in --help (app.py ORDER without the
// retired commands).
var order = []string{
	"class", "search", "guide", "guide-search", "devguide", "pdf", "method",
	"source", "package", "slots", "module", "type", "find", "devguide-search", "stats",
	"source-grep", "coverage", "freshness", "rebuild-all", "selftest", "versions",
}

// retired are the niagara_help.py commands the Go port does not carry
// (decision D32: no recorded use). Each still parses to one stderr line
// pointing at docs/retired-commands.md, and exit 2.
var retired = map[string]bool{}

func init() {
	for _, n := range []string{
		"hierarchy", "xref", "implementors", "related", "module-deps",
		"compare", "annotations", "profile", "snippet", "callers", "overrides",
		"contract", "inherited-methods", "compatible", "graph", "api", "hotspots",
		"chain", "module-summary", "archetypes", "orphans", "checklist",
		"action-params", "diff-api", "build-call-index", "interface-hierarchy",
		"inherited-slots", "topics", "recipe", "event-flow", "examples", "fuzzy",
		"enum", "method-sig", "export", "export-html", "export-module", "repl",
	} {
		retired[n] = true
	}
}

// retiredMessage is the line a retired command prints on stderr.
func retiredMessage(name string) string {
	return "nh: '" + name + "' was retired in the Go port (no recorded use); see docs/retired-commands.md"
}

// epilog is the --help example list (app.py build_parser), without the
// examples of retired commands.
const epilog = "Examples:\n" +
	"  %(prog)s class BWebServlet\n" +
	"  %(prog)s search \"BAlarm*\"\n" +
	"  %(prog)s guide bacnet\n" +
	"  %(prog)s guide-search \"discovery timeout\"\n" +
	"  %(prog)s devguide \"component model\"\n" +
	"  %(prog)s pdf \"modbus configuration\"\n" +
	"  %(prog)s method doGet\n" +
	"  %(prog)s source BAlarmService --code\n" +
	"  %(prog)s source-grep \"getSession()\" -n 20\n" +
	"  %(prog)s package javax.baja.alarm\n" +
	"  %(prog)s slots BAlarmClass\n" +
	"  %(prog)s slots faultCause\n" +
	"  %(prog)s slots BAlarmClass --inherited\n" +
	"  %(prog)s module kitControl-rt\n" +
	"  %(prog)s type kitControl:BooleanLatch\n" +
	"  %(prog)s find \"setpoint reference\"\n" +
	"  %(prog)s devguide-search \"web servlet\"\n" +
	"  %(prog)s versions add /mnt/c/Niagara/Niagara-4.16.0.1\n"

// commandNames lists the registered commands in --help order; a command
// missing from order is listed last rather than lost (app.py build_parser).
func commandNames() []string {
	var names []string
	listed := map[string]bool{}
	for _, n := range order {
		if registry[n] != nil {
			names = append(names, n)
			listed[n] = true
		}
	}
	var rest []string
	for n := range registry {
		if !listed[n] {
			rest = append(rest, n)
		}
	}
	sort.Strings(rest)
	return append(names, rest...)
}

// buildParser is app.py build_parser. The prog stays "niagara_help" so help,
// usage and errors are byte-identical to the Python CLI.
func buildParser() *argparse.Parser {
	p := argparse.New(argparse.Config{
		Prog:        "niagara_help",
		Description: "Niagara N4 Documentation Navigator",
		Formatter:   argparse.RawDescriptionHelpFormatter,
		Epilog:      epilog,
	})
	sub := p.AddSubparsers(argparse.SubparsersConfig{Dest: "command", Help: "Available commands"})
	for _, name := range commandNames() {
		c := registry[name]
		cp := sub.AddParser(name, argparse.SubparserConfig{Help: c.helpText()})
		if c.args != nil {
			c.args(cp)
		}
	}
	p.Add(argparse.Arg{Dest: "_niagara_version_help", Metavar: "VERSION",
		Help: "Niagara version to query (default from versions/registry.json; " +
			"env NIAGARA_HELP_VERSION). Accepted anywhere on the command line."},
		"--niagara-version")
	return p
}

// Argument helpers for the add_args functions of the command files.

// positional is add_argument(name, help=help).
func positional(p *argparse.Parser, name, help string) {
	p.Add(argparse.Arg{Help: help}, name)
}

// optionalPositional is add_argument(name, nargs="?", default=def, help=help).
func optionalPositional(p *argparse.Parser, name string, def any, help string) {
	p.Add(argparse.Arg{Nargs: argparse.Optional, Default: def, Help: help}, name)
}

// flag is add_argument(*names, action="store_true", help=help).
func flag(p *argparse.Parser, help string, names ...string) {
	p.Add(argparse.Arg{Action: argparse.StoreTrue, Help: help}, names...)
}

// intOpt is add_argument(*names, type=int, default=def, help=help) for a
// count: a negative one is a usage error (nonNegativeInt; Python's type=int
// let -n -5 count from the end).
func intOpt(p *argparse.Parser, def any, help string, names ...string) {
	p.Add(argparse.Arg{Type: nonNegativeInt, Default: def, Help: help}, names...)
}

// nonNegativeInt is type=int that refuses a value below 0, named "int" in
// argparse's messages as Python's int is.
var nonNegativeInt = &argparse.Type{Name: "int", Convert: func(s string) (any, error) {
	v, err := argparse.Int.Convert(s)
	if err != nil {
		return nil, err
	}
	n, ok := v.(int)
	if !ok { // a *big.Int beyond int's range
		if b, isBig := v.(*big.Int); isBig && b.Sign() < 0 {
			return nil, argparse.NewTypeError("must be 0 or more, not %s", b)
		}
		return math.MaxInt, nil
	}
	if n < 0 {
		return nil, argparse.NewTypeError("must be 0 or more, not %d", n)
	}
	return n, nil
}}

// choices converts option values to argparse choices.
func choices(vals ...string) []any {
	out := make([]any, len(vals))
	for i, v := range vals {
		out[i] = v
	}
	return out
}
