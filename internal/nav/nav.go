// Package nav is module-nav (module-navigator tools/module_nav.py): the parser built
// from the command registry, base directory detection and dispatch. Each command group
// lives in its own package under nav/ and registers itself; importing them here is
// what makes them part of the CLI.
package nav

import (
	"fmt"
	"os"
	"strings"

	"github.com/angeles725/niagara-go/internal/argparse"
	"github.com/angeles725/niagara-go/internal/cli"
	_ "github.com/angeles725/niagara-go/internal/nav/architecture" // registers its commands
	_ "github.com/angeles725/niagara-go/internal/nav/audits"
	_ "github.com/angeles725/niagara-go/internal/nav/domain"
	_ "github.com/angeles725/niagara-go/internal/nav/flow"
	_ "github.com/angeles725/niagara-go/internal/nav/inventory"
	_ "github.com/angeles725/niagara-go/internal/nav/lookup"
	_ "github.com/angeles725/niagara-go/internal/nav/patching"
	_ "github.com/angeles725/niagara-go/internal/nav/quality"
	"github.com/angeles725/niagara-go/internal/nav/registry"
	_ "github.com/angeles725/niagara-go/internal/nav/security"
	_ "github.com/angeles725/niagara-go/internal/nav/session"
	_ "github.com/angeles725/niagara-go/internal/nav/source"
	_ "github.com/angeles725/niagara-go/internal/nav/structure"
	_ "github.com/angeles725/niagara-go/internal/nav/ui"
)

// Main runs one module-nav command line and returns the process exit code.
func Main(args []string, s cli.Stdio) int {
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
	if baseDir == "" {
		baseDir = DetectBaseDir()
	}
	if baseDir == "" {
		fmt.Fprintln(s.Out, "ERROR: Cannot detect base directory.")
		fmt.Fprintln(s.Out, "Use --base-dir or run from module-navigator/tools/")
		return cli.ExitNoMatch
	}
	c, _ := registry.Lookup(name)
	if c.Run == nil {
		fmt.Fprintf(s.Err, "mnav: '%s' is not ported yet\n", name)
		return cli.ExitCannotSearch
	}
	return c.Run(&registry.Env{BaseDir: baseDir, Stdio: s}, ns)
}

// BuildParser is build_parser(): the module-nav parser with every registered command.
func BuildParser() (*argparse.Parser, error) {
	parser := argparse.New(argparse.Config{
		Prog:        "module-nav",
		Usage:       "module-nav [-h] [--base-dir DIR] <command> [<args>]",
		Description: "Module Navigator - CLI for Niagara N4 decompiled modules",
		Formatter:   argparse.RawDescriptionHelpFormatter,
	})
	parser.Add(argparse.Arg{Help: "Base directory (auto-detected if omitted)"}, "--base-dir", "-d")
	// The grouped list in the epilog replaces argparse's one-line {a,b,...} usage
	// and its flat command list.
	sub := parser.AddSubparsers(argparse.SubparsersConfig{Dest: "command", Metavar: "<command>", Help: argparse.Suppress})
	if err := registry.AddParsers(sub, Order); err != nil {
		return nil, err
	}
	epilog, err := groupedCommands()
	if err != nil {
		return nil, err
	}
	parser.SetEpilog(epilog)
	return parser, nil
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

// executable is os.Executable, replaceable in tests.
var executable = os.Executable
