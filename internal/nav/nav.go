package nav

import (
	"bufio"
	"fmt"
	"io"
	"strings"

	"github.com/angeles725/niagara-go/internal/argparse"
	"github.com/angeles725/niagara-go/internal/cli"
	_ "github.com/angeles725/niagara-go/internal/nav/audits" // registers its commands
	"github.com/angeles725/niagara-go/internal/nav/core"
	_ "github.com/angeles725/niagara-go/internal/nav/domain"
	_ "github.com/angeles725/niagara-go/internal/nav/flow"
	_ "github.com/angeles725/niagara-go/internal/nav/inventory"
	_ "github.com/angeles725/niagara-go/internal/nav/lookup"
	"github.com/angeles725/niagara-go/internal/nav/registry"
	_ "github.com/angeles725/niagara-go/internal/nav/security"
	_ "github.com/angeles725/niagara-go/internal/nav/source"
	_ "github.com/angeles725/niagara-go/internal/nav/structure"
)

// Main runs one mnav command line and returns the process exit code: a
// reindex line goes to the index rebuild plugged in with UseReindex
// (reindex.go), any other to run.
func Main(args []string, s cli.Stdio) int {
	if rest, ok := reindexArgs(args); ok {
		if reindexMain == nil {
			fmt.Fprintln(s.Err, "mnav: the index builder is not linked into this binary")
			return cli.ExitCannotSearch
		}
		return reindexMain(rest, s)
	}
	return run("", args, s)
}

// run is Main, or RunAt when navDir (absolute) pins the checkout: the default
// base directory and the runtime's corpus location.
func run(navDir string, args []string, s cli.Stdio) int {
	if name := commandWord(args); retired[name] {
		fmt.Fprintf(s.Err, "mnav: '%s' was retired in the Go port (no recorded use); see docs/retired-commands.md\n", name)
		return cli.ExitCannotSearch
	}
	parser, err := BuildParser()
	if err != nil {
		fmt.Fprintf(s.Err, "mnav: %v\n", err)
		return cli.ExitCannotSearch
	}
	ns, code, done := parser.Parse(args, s)
	if done {
		return code
	}
	name := ns.String("command")
	if name == "" {
		parser.PrintHelp(s.Out)
		return cli.ExitFound
	}
	baseDir := ns.String("base_dir")
	if baseDir == "" && navDir != "" {
		baseDir = navDir
	}
	if baseDir == "" {
		var err error
		if baseDir, err = DetectBaseDir(); err != nil {
			fmt.Fprintf(s.Err, "mnav: %v\n", err)
			return cli.ExitCannotSearch
		}
	}
	if baseDir == "" {
		fmt.Fprintln(s.Err, "ERROR: Cannot detect base directory.")
		fmt.Fprintln(s.Err, "Use --base-dir or run from module-navigator/tools/")
		return cli.ExitCannotSearch
	}
	c, _ := registry.Lookup(name)
	if c.Run == nil {
		fmt.Fprintf(s.Err, "mnav: '%s' is not ported yet\n", name)
		return cli.ExitCannotSearch
	}
	out := bufio.NewWriter(s.Out)
	errw := orderedErr{out, s.Err}
	rt := core.NewAt(navDir, baseDir, out, errw)
	defer rt.Close()
	status := c.Run(&registry.Env{Stdio: cli.Stdio{In: s.In, Out: out, Err: errw}, Runtime: rt}, ns)
	if err := out.Flush(); err != nil {
		fmt.Fprintf(s.Err, "mnav: cannot write to stdout: %v\n", err)
		return cli.ExitCannotSearch
	}
	return status
}

// retired is the set of Retired.
var retired = func() map[string]bool {
	m := map[string]bool{}
	for _, n := range Retired {
		m[n] = true
	}
	return m
}()

// commandWord is the command name of a module-nav command line: the first
// word after the global options (--base-dir/-d DIR in any of argparse's
// spellings, "--"); "" when an option comes first that is not one of them
// (help or a usage error, which the parser reports).
func commandWord(args []string) string {
	if i := commandIndex(args); i >= 0 {
		return args[i]
	}
	return ""
}

// commandIndex is the index in args of commandWord, -1 when there is none.
func commandIndex(args []string) int {
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--":
			if i+1 < len(args) {
				return i + 1
			}
			return -1
		case a == "-d":
			i++
		case strings.HasPrefix(a, "-d"):
		case strings.HasPrefix(a, "--") && len(a) > 2:
			name, _, hasValue := strings.Cut(a, "=")
			if !strings.HasPrefix("--base-dir", name) {
				return -1
			}
			if !hasValue {
				i++
			}
		case strings.HasPrefix(a, "-") && a != "-":
			return -1
		default:
			return i
		}
	}
	return -1
}

// orderedErr is a command's stderr: it flushes the buffered stdout before each
// write, so a terminal showing both streams sees them in the order the command
// wrote them, as with Python's line-buffered stdout. A failed flush is sticky
// in the bufio.Writer and reported when Main flushes at the end.
type orderedErr struct {
	out *bufio.Writer
	err io.Writer
}

func (w orderedErr) Write(p []byte) (int, error) {
	w.out.Flush()
	return w.err.Write(p)
}

// BuildParser is build_parser(): the module-nav parser with every registered command.
func BuildParser() (*argparse.Parser, error) {
	parser := argparse.New(argparse.Config{
		Prog:        "module-nav",
		Usage:       "module-nav [-h] [--base-dir DIR] <command> [<args>]",
		Description: "Module Navigator - CLI for Niagara N4 decompiled modules",
		Formatter:   argparse.RawDescriptionHelpFormatter,
	})
	addGlobalOptions(parser)
	// The grouped list in the epilog replaces argparse's one-line {a,b,...} usage
	// and its flat command list.
	sub := parser.AddSubparsers(argparse.SubparsersConfig{Dest: "command", Metavar: "<command>", Help: argparse.Suppress})
	if err := registry.AddParsers(sub, Order, Retired); err != nil {
		return nil, err
	}
	epilog, err := groupedCommands()
	if err != nil {
		return nil, err
	}
	parser.SetEpilog(epilog)
	return parser, nil
}

// addGlobalOptions adds module-nav's options before the command word.
func addGlobalOptions(p *argparse.Parser) {
	p.Add(argparse.Arg{Help: "Base directory (auto-detected if omitted)"}, "--base-dir", "-d")
}

// groupedCommands is the --help epilog: every command under its group heading, one
// per line with its help. A group without a heading is an error, never a silently
// dropped command.
func groupedCommands() (string, error) {
	byGroup := map[string][]string{}
	width := 0
	for _, name := range Order {
		c, _ := registry.Lookup(name)
		byGroup[c.Group] = append(byGroup[c.Group], name)
		width = max(width, len(name))
	}
	width += 2
	headed := map[string]bool{}
	for _, g := range Groups {
		headed[g.Name] = true
	}
	for g := range byGroup {
		if !headed[g] {
			return "", fmt.Errorf("command group without a heading: %s", g)
		}
	}
	out := []string{"commands (module-nav <command> --help for its options):"}
	for _, g := range Groups {
		out = append(out, "", "  "+g.Title+":")
		for _, name := range byGroup[g.Name] {
			c, _ := registry.Lookup(name)
			out = append(out, fmt.Sprintf("    %-*s%s", width, name, c.Help))
		}
	}
	return strings.Join(out, "\n"), nil
}
