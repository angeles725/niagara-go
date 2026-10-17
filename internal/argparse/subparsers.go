package argparse

import (
	"strings"

	"github.com/angeles725/niagara-go/internal/pycompat"
)

// Subparsers is the action add_subparsers returns (_SubParsersAction).
type Subparsers struct {
	action     *Action
	progPrefix string
	names      []string // command names and aliases in registration order
	parsers    map[string]*Parser
	choices    []*Action // the help lines (_ChoicesPseudoAction)
}

// AddSubparsers is parser.add_subparsers(**c). It panics when called twice.
func (p *Parser) AddSubparsers(c SubparsersConfig) *Subparsers {
	if p.subparsers != nil {
		panic("argparse: cannot have multiple subparser arguments")
	}
	group := p.positionals
	if c.Title != "" || c.Description != "" {
		title := c.Title
		if title == "" {
			title = "subcommands"
		}
		group = p.AddArgumentGroup(title, c.Description)
	}
	prog := c.Prog
	if prog == "" {
		// the usage of this parser without its optional options and "usage: "
		var acts []*Action
		for _, a := range p.actions {
			if a.isOptional() && a.Required {
				acts = append(acts, a)
			}
		}
		for _, a := range p.actions {
			if !a.isOptional() {
				acts = append(acts, a)
			}
		}
		f := newFormatter(p)
		f.addUsage("", acts, p.mutexGroups, "")
		prog = pycompat.Strip(f.formatHelp())
	}
	dest := c.Dest
	if dest == "" {
		dest = Suppress
	}
	s := &Subparsers{progPrefix: prog, parsers: map[string]*Parser{}}
	s.action = &Action{kind: parsers, nargs: nargsPar, Dest: dest, Required: c.Required,
		Help: c.Help, Metavar: c.Metavar, sub: s}
	p.checkHelp(s.action)
	group.addAction(s.action)
	p.subparsers = s
	return s
}

// SetRequired assigns the action's required attribute (`sub.required = True`).
func (s *Subparsers) SetRequired(required bool) { s.action.Required = required }

// AddParser is add_parser(name, **c): a new Parser run for this command and its aliases.
func (s *Subparsers) AddParser(name string, c SubparserConfig) *Parser {
	cfg := c.Config
	if cfg.Prog == "" {
		cfg.Prog = s.progPrefix + " " + name
	}
	if _, dup := s.parsers[name]; dup {
		panic("argparse: conflicting subparser: " + name)
	}
	for _, alias := range c.Aliases {
		if _, dup := s.parsers[alias]; dup {
			panic("argparse: conflicting subparser alias: " + alias)
		}
	}
	var choice *Action
	if c.Help != "" || c.Listed {
		metavar := name
		if len(c.Aliases) > 0 {
			metavar += " (" + strings.Join(c.Aliases, ", ") + ")"
		}
		choice = &Action{Dest: name, Help: c.Help, Metavar: metavar}
		s.choices = append(s.choices, choice)
	}
	sp := New(cfg)
	if choice != nil {
		sp.checkHelp(choice)
	}
	s.parsers[name] = sp
	s.names = append(s.names, name)
	for _, alias := range c.Aliases {
		s.parsers[alias] = sp
		s.names = append(s.names, alias)
	}
	return sp
}

// Parser returns the parser registered for a command name or alias.
func (s *Subparsers) Parser(name string) *Parser { return s.parsers[name] }

func (s *Subparsers) choiceValues() []any {
	out := make([]any, len(s.names))
	for i, n := range s.names {
		out[i] = n
	}
	return out
}

// checkHelp is _check_help: a help string that cannot be expanded is a definition error.
func (p *Parser) checkHelp(a *Action) {
	if a.Help == "" {
		return
	}
	if _, err := newFormatter(p).expandHelp(a); err != nil {
		panic("argparse: badly formed help string: " + err.Error())
	}
}
