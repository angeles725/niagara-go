package help

import (
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

// commandGroups are the cmd_*.py modules, in their import order.
var commandGroups = [][]command{
	versionCommands,
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

// order is the order of the subcommands in --help (app.py ORDER).
var order = []string{
	"class", "search", "hierarchy", "guide", "guide-search", "devguide", "pdf",
	"module", "method", "source", "package", "xref", "slots", "implementors",
	"find", "related", "devguide-search", "stats", "source-grep",
	"inherited-slots", "examples", "method-sig", "enum", "module-deps",
	"compare", "annotations", "profile", "snippet", "callers", "overrides",
	"contract", "inherited-methods", "compatible", "graph", "api", "hotspots",
	"chain", "fuzzy", "module-summary", "export", "interface-hierarchy",
	"archetypes", "orphans", "recipe", "checklist", "topics", "action-params",
	"event-flow", "export-html", "export-module", "coverage", "diff-api",
	"build-call-index", "freshness", "rebuild-all", "selftest", "repl",
	"versions",
}

// epilog is the --help example list (app.py build_parser).
const epilog = "Examples:\n" +
	"  %(prog)s class BWebServlet\n" +
	"  %(prog)s search \"BAlarm*\"\n" +
	"  %(prog)s hierarchy BComponent --depth 2\n" +
	"  %(prog)s guide bacnet\n" +
	"  %(prog)s guide-search \"discovery timeout\"\n" +
	"  %(prog)s devguide \"component model\"\n" +
	"  %(prog)s pdf \"modbus configuration\"\n" +
	"  %(prog)s module alarm\n" +
	"  %(prog)s method doGet\n" +
	"  %(prog)s method-sig BAlarmRecord\n" +
	"  %(prog)s source BAlarmService --code\n" +
	"  %(prog)s source-grep \"getSession()\" -n 20\n" +
	"  %(prog)s package javax.baja.alarm\n" +
	"  %(prog)s xref BAlarmService\n" +
	"  %(prog)s slots BAlarmClass\n" +
	"  %(prog)s slots faultCause\n" +
	"  %(prog)s slots BAlarmClass --inherited\n" +
	"  %(prog)s implementors BIAlarmSource\n" +
	"  %(prog)s find \"setpoint reference\"\n" +
	"  %(prog)s related BAlarmService\n" +
	"  %(prog)s devguide-search \"web servlet\"\n" +
	"  %(prog)s examples BWebServlet\n" +
	"  %(prog)s enum BAckState\n" +
	"  %(prog)s module-deps alarm-rt\n" +
	"  %(prog)s compare BWebServlet BServletView\n" +
	"  %(prog)s annotations NiagaraAction\n" +
	"  %(prog)s profile BAlarmService\n" +
	"  %(prog)s snippet BAlarmService doRouteAlarm\n" +
	"  %(prog)s callers doRouteAlarm\n" +
	"  %(prog)s overrides BOrionAlarmService\n" +
	"  %(prog)s contract BIAlarmSource\n" +
	"  %(prog)s inherited-methods BNumericWritable\n" +
	"  %(prog)s compatible BStatusNumeric\n" +
	"  %(prog)s graph BAlarmService\n" +
	"  %(prog)s api BAlarmService\n" +
	"  %(prog)s hotspots\n" +
	"  %(prog)s chain doRouteAlarm --depth 3\n" +
	"  %(prog)s fuzzy \"alarm service route\"\n" +
	"  %(prog)s module-summary alarm-rt\n" +
	"  %(prog)s export BAlarmService report.md\n" +
	"  %(prog)s versions add /mnt/c/Niagara/Niagara-4.16.0.1\n" +
	"  %(prog)s repl\n"

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
