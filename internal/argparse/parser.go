package argparse

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode"
)

// Action is one added argument (argparse.Action). It is shared, like in Python, between a
// parent parser and the parsers that list it in Parents.
type Action struct {
	OptionStrings []string
	Dest          string
	Default       any
	Required      bool
	Help          string
	Metavar       string // "" is None

	kind         ActionKind
	nargs        int
	nargsZero    bool // nargs=0: the store_true family and Custom NoArgs
	constant     any
	typ          *Type
	choices      []any
	metavarTuple []string
	version      string
	call         func(*Call) error
	sub          *Subparsers // the add_subparsers action
}

func (a *Action) isOptional() bool { return len(a.OptionStrings) > 0 }

// Group is an argument group (add_argument_group); the parser's positional and option
// sections are groups too.
type Group struct {
	p           *Parser
	title       string
	hasTitle    bool
	description string
	actions     []*Action
}

// MutexGroup is add_mutually_exclusive_group.
type MutexGroup struct {
	p         *Parser
	container adder
	Required  bool
	actions   []*Action
}

// adder is a container's _add_action.
type adder interface{ addAction(*Action) *Action }

// Parser is argparse.ArgumentParser.
type Parser struct {
	cfg         Config
	prog        string
	actions     []*Action
	optIndex    map[string]*Action
	optOrder    []string // option strings in insertion order (dict order)
	groups      []*Group
	mutexGroups []*MutexGroup
	defaults    map[string]any
	defaultKeys []string
	positionals *Group
	optionals   *Group
	subparsers  *Subparsers
	negOptions  bool // an option string looks like a negative number
}

// New is ArgumentParser(**cfg).
func New(cfg Config) *Parser {
	p := &Parser{cfg: cfg, prog: cfg.Prog, optIndex: map[string]*Action{}, defaults: map[string]any{}}
	if p.prog == "" {
		p.prog = filepath.Base(os.Args[0])
	}
	p.positionals = p.AddArgumentGroup("positional arguments", "")
	p.optionals = p.AddArgumentGroup("options", "")
	if !cfg.NoHelp {
		p.Add(Arg{Action: Help, Default: Suppress, Help: "show this help message and exit"}, "-h", "--help")
	}
	for _, parent := range cfg.Parents {
		p.addContainerActions(parent)
		for _, k := range parent.defaultKeys {
			p.setDefault(k, parent.defaults[k])
		}
	}
	return p
}

// Prog is the parser's prog.
func (p *Parser) Prog() string { return p.prog }

// SetEpilog assigns parser.epilog.
func (p *Parser) SetEpilog(s string) { p.cfg.Epilog = s }

// SetDescription assigns parser.description.
func (p *Parser) SetDescription(s string) { p.cfg.Description = s }

// SetUsage assigns parser.usage.
func (p *Parser) SetUsage(s string) { p.cfg.Usage = s }

func (p *Parser) setDefault(k string, v any) {
	if _, ok := p.defaults[k]; !ok {
		p.defaultKeys = append(p.defaultKeys, k)
	}
	p.defaults[k] = v
}

// SetDefaults is parser.set_defaults(**kv): namespace values for dests no argument sets,
// and the new default of the arguments that do.
func (p *Parser) SetDefaults(kv map[string]any) {
	for k, v := range kv {
		p.setDefault(k, v)
	}
	for _, a := range p.actions {
		if v, ok := kv[a.Dest]; ok {
			a.Default = v
		}
	}
}

// AddArgumentGroup is add_argument_group(title, description); "" means None.
func (p *Parser) AddArgumentGroup(title, description string) *Group {
	g := &Group{p: p, title: title, hasTitle: title != "", description: description}
	p.groups = append(p.groups, g)
	return g
}

// AddMutuallyExclusiveGroup is add_mutually_exclusive_group(required=required).
func (p *Parser) AddMutuallyExclusiveGroup(required bool) *MutexGroup {
	m := &MutexGroup{p: p, container: p, Required: required}
	p.mutexGroups = append(p.mutexGroups, m)
	return m
}

// AddMutuallyExclusiveGroup adds a mutually exclusive group inside the group.
func (g *Group) AddMutuallyExclusiveGroup(required bool) *MutexGroup {
	m := &MutexGroup{p: g.p, container: g, Required: required}
	g.p.mutexGroups = append(g.p.mutexGroups, m)
	return m
}

// Add is add_argument(*names, **a). It panics on the definition errors Python raises.
func (p *Parser) Add(a Arg, names ...string) *Action { return p.addAction(p.newAction(a, names)) }

// Add is add_argument on the group.
func (g *Group) Add(a Arg, names ...string) *Action { return g.addAction(g.p.newAction(a, names)) }

// Add is add_argument on the mutually exclusive group.
func (m *MutexGroup) Add(a Arg, names ...string) *Action { return m.addAction(m.p.newAction(a, names)) }

// addAction is ArgumentParser._add_action: options and positionals go to their section.
func (p *Parser) addAction(a *Action) *Action {
	if a.isOptional() {
		return p.optionals.addAction(a)
	}
	return p.positionals.addAction(a)
}

func (g *Group) addAction(a *Action) *Action {
	g.p.registerAction(a)
	g.actions = append(g.actions, a)
	return a
}

func (m *MutexGroup) addAction(a *Action) *Action {
	if a.Required {
		panic("argparse: mutually exclusive arguments must be optional")
	}
	m.container.addAction(a)
	m.actions = append(m.actions, a)
	return a
}

// registerAction is _ActionsContainer._add_action (conflict_handler='error').
func (p *Parser) registerAction(a *Action) {
	var conflicts []string
	for _, s := range a.OptionStrings {
		if _, ok := p.optIndex[s]; ok {
			conflicts = append(conflicts, s)
		}
	}
	if len(conflicts) > 0 {
		word := "string"
		if len(conflicts) > 1 {
			word = "strings"
		}
		name, _ := actionName(a)
		panic(fmt.Sprintf("argparse: argument %s: conflicting option %s: %s",
			name, word, strings.Join(conflicts, ", ")))
	}
	p.actions = append(p.actions, a)
	for _, s := range a.OptionStrings {
		p.optIndex[s] = a
		p.optOrder = append(p.optOrder, s)
		if negativeNumber(s) {
			p.negOptions = true
		}
	}
}

// addContainerActions is _add_container_actions: copy a parent's groups and actions.
func (p *Parser) addContainerActions(parent *Parser) {
	titleGroup := map[string]*Group{}
	key := func(g *Group) string {
		if !g.hasTitle {
			return "\x00none"
		}
		return g.title
	}
	for _, g := range p.groups {
		if _, dup := titleGroup[key(g)]; dup {
			panic(fmt.Sprintf("argparse: cannot merge actions - two groups are named %s", pyRepr(g.title)))
		}
		titleGroup[key(g)] = g
	}
	groupOf := map[*Action]adder{}
	for _, g := range parent.groups {
		if _, ok := titleGroup[key(g)]; !ok {
			titleGroup[key(g)] = p.AddArgumentGroup(g.title, g.description)
		}
		for _, a := range g.actions {
			groupOf[a] = titleGroup[key(g)]
		}
	}
	for _, m := range parent.mutexGroups {
		var mg *MutexGroup
		if pg, ok := m.container.(*Group); ok {
			mg = titleGroup[key(pg)].AddMutuallyExclusiveGroup(m.Required)
		} else {
			mg = p.AddMutuallyExclusiveGroup(m.Required)
		}
		for _, a := range m.actions {
			groupOf[a] = mg
		}
	}
	for _, a := range parent.actions {
		if g, ok := groupOf[a]; ok {
			g.addAction(a)
		} else {
			p.addAction(a)
		}
	}
}

// newAction is the validation and keyword defaulting of add_argument.
func (p *Parser) newAction(arg Arg, names []string) *Action {
	a := &Action{kind: arg.Action, nargs: arg.Nargs, constant: arg.Const, Default: arg.Default,
		typ: arg.Type, choices: arg.Choices, Required: arg.Required, Help: arg.Help,
		Metavar: arg.Metavar, metavarTuple: arg.MetavarTuple, version: arg.Version, call: arg.Call}
	if len(names) == 0 || (len(names) == 1 && !strings.HasPrefix(names[0], "-")) {
		if len(names) == 0 {
			panic("argparse: a positional argument needs a name")
		}
		if arg.Dest != "" {
			panic("argparse: dest supplied twice for positional argument, did you mean metavar?")
		}
		if arg.Required {
			panic("argparse: 'required' is an invalid argument for positionals")
		}
		a.Dest = names[0]
		a.Required = a.nargs != Optional && a.nargs != ZeroOrMore
	} else {
		a.OptionStrings = append([]string(nil), names...)
		a.Dest = arg.Dest
		if a.Dest == "" {
			a.Dest = inferDest(names)
		}
	}
	if a.Default == nil {
		if v, ok := p.defaults[a.Dest]; ok {
			a.Default = v
		}
	}
	switch a.kind {
	case StoreConst, StoreTrue, StoreFalse, AppendConst, Count, Help, Version:
		a.nargs, a.nargsZero, a.typ, a.choices = 0, true, nil, nil
		if a.kind == StoreTrue || a.kind == StoreFalse {
			a.constant = a.kind == StoreTrue
			if a.Default == nil {
				a.Default = a.kind == StoreFalse
			}
		}
		if a.kind == Version {
			if a.Help == "" {
				a.Help = "show program's version number and exit"
			}
			if a.Default == nil {
				a.Default = Suppress
			}
		}
	case Store, Append, Extend:
		if a.nargs == NoArgs {
			panic("argparse: nargs for store actions must be != 0")
		}
		if a.constant != nil && a.nargs != Optional {
			panic("argparse: nargs must be '?' to supply const")
		}
	case Custom:
		if a.call == nil {
			panic("argparse: a Custom action needs Call")
		}
		if a.nargs == NoArgs {
			a.nargs, a.nargsZero = 0, true
		}
	default:
		panic(fmt.Sprintf("argparse: unknown action %d", a.kind))
	}
	if !a.isOptional() && a.nargsZero {
		panic("argparse: action is not valid for positional arguments")
	}
	if a.metavarTuple != nil && !metavarFits(a) {
		panic("argparse: length of metavar tuple does not match nargs")
	}
	p.checkHelp(a)
	return a
}

// inferDest is '--foo-bar' -> 'foo_bar', '-x' -> 'x', preferring the first long option.
func inferDest(names []string) string {
	dest, priority := "", 0
	for _, s := range names {
		switch {
		case len(s) <= 2:
			if priority < 1 {
				dest, priority = strings.TrimLeft(s, "-"), 1
			}
		case s[1] != '-':
			if priority < 2 {
				dest, priority = strings.TrimLeft(s, "-"), 2
			}
		default:
			return strings.ReplaceAll(strings.TrimLeft(s, "-"), "-", "_")
		}
	}
	if dest == "" {
		panic(fmt.Sprintf("argparse: dest= is required for options like %s", strings.Join(names, ", ")))
	}
	return strings.ReplaceAll(dest, "-", "_")
}

// metavarFits is the TypeError check of _format_args for a tuple metavar.
func metavarFits(a *Action) bool {
	n := len(a.metavarTuple)
	switch {
	case a.nargs == ZeroOrMore:
		return n == 1 || n == 2
	case a.nargs == OneOrMore:
		return n == 2
	case a.nargs > 0:
		return n == a.nargs
	}
	return n == 1
}

// negativeNumber is _negative_number_matcher: '-' then an optional '.' and a digit.
func negativeNumber(s string) bool {
	r := []rune(s)
	if len(r) < 2 || r[0] != '-' {
		return false
	}
	if r[1] == '.' {
		return len(r) > 2 && isUnicodeDigit(r[2])
	}
	return isUnicodeDigit(r[1])
}

// actionName is _get_action_name.
func actionName(a *Action) (string, bool) {
	switch {
	case a == nil:
		return "", false
	case a.isOptional():
		return strings.Join(a.OptionStrings, "/"), true
	case a.metavarTuple != nil:
		m := a.metavarTuple
		if (a.nargs == ZeroOrMore && len(m) == 2) || a.nargs == OneOrMore {
			return m[0] + "[, " + m[1] + "]", true
		}
		return strings.Join(m, ", "), true
	case a.Metavar != "" && a.Metavar != Suppress:
		return a.Metavar, true
	case a.Dest != "" && a.Dest != Suppress:
		return a.Dest, true
	}
	if c := a.choiceValues(); len(c) > 0 {
		return "{" + joinStr(c, ",") + "}", true
	}
	return "", false
}

func (a *Action) choiceValues() []any {
	if a.sub != nil {
		return a.sub.choiceValues()
	}
	return a.choices
}

func joinStr(vals []any, sep string) string {
	parts := make([]string, len(vals))
	for i, v := range vals {
		parts[i] = pyStr(v)
	}
	return strings.Join(parts, sep)
}

func isUnicodeDigit(r rune) bool { return unicode.IsDigit(r) }

func isSuppress(v any) bool {
	s, ok := v.(string)
	return ok && s == Suppress
}
