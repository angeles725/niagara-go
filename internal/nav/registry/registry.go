// Package registry holds every module-nav command: its parser declaration and, once
// ported, its implementation. Each command group package registers its commands from
// init, so adding a command never edits a central file beyond the help order in nav.
package registry

import (
	"fmt"
	"sort"

	"github.com/angeles725/niagara-go/internal/argparse"
	"github.com/angeles725/niagara-go/internal/cli"
	"github.com/angeles725/niagara-go/internal/nav/core"
)

// Env is what a running command receives besides its parsed arguments: its
// stdio (stdout buffered until the command returns) and the runtime with the
// base directory, outcomes and loaded indexes.
type Env struct {
	Stdio cli.Stdio
	*core.Runtime
}

// Run executes a parsed command and returns its exit code.
type Run func(env *Env, ns *argparse.Namespace) int

// Command is one subcommand (module_nav_lib/commands/<group>.py @command).
type Command struct {
	Name        string
	Group       string // set by Register
	Help        string // add_parser(help=...): the line in the grouped --help list
	Description string
	Epilog      string
	Args        func(p *argparse.Parser) // add_argument calls; nil when it takes none
	Run         Run                      // nil until the command is ported
}

var commands = map[string]Command{}

// Register adds the commands of group. It panics when a name is registered twice.
func Register(group string, cmds []Command) {
	for _, c := range cmds {
		if _, dup := commands[c.Name]; dup {
			panic("registry: command registered twice: " + c.Name)
		}
		c.Group = group
		commands[c.Name] = c
	}
}

// Lookup returns the command registered under name.
func Lookup(name string) (Command, bool) {
	c, ok := commands[name]
	return c, ok
}

// Names returns every registered command name, sorted.
func Names() []string {
	out := make([]string, 0, len(commands))
	for n := range commands {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}

// AddParsers adds the commands in order to sub (registry.add_commands); order must
// name every registered command exactly once.
func AddParsers(sub *argparse.Subparsers, order []string) error {
	seen := map[string]bool{}
	for _, name := range order {
		c, ok := commands[name]
		if !ok {
			return fmt.Errorf("registry: order names an unknown command: %s", name)
		}
		if seen[name] {
			return fmt.Errorf("registry: order names a command twice: %s", name)
		}
		seen[name] = true
		p := sub.AddParser(name, argparse.SubparserConfig{
			Config: argparse.Config{Description: c.Description, Epilog: c.Epilog},
			Help:   c.Help, Listed: true,
		})
		if c.Args != nil {
			c.Args(p)
		}
	}
	for _, name := range Names() {
		if !seen[name] {
			return fmt.Errorf("registry: command missing from the help order: %s", name)
		}
	}
	return nil
}
