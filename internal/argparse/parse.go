package argparse

import (
	"io"
	"strings"

	"github.com/angeles725/niagara-go/internal/cli"
	"github.com/angeles725/niagara-go/internal/pycompat"
)

// runCtx carries the output streams through one parse (Python's sys.stdout/sys.stderr).
type runCtx struct{ out, err io.Writer }

func newCtx(s cli.Stdio) *runCtx {
	ctx := &runCtx{out: s.Out, err: s.Err}
	if ctx.out == nil {
		ctx.out = io.Discard
	}
	if ctx.err == nil {
		ctx.err = io.Discard
	}
	return ctx
}

// exitStatus is a SystemExit unwinding the parse: help, version, a usage error already
// printed by the parser that raised it, or a Custom action's ExitError.
type exitStatus struct{ code int }

func (e *exitStatus) Error() string { return "exit" }

// Parse is parse_args(args). When done is true the parser already wrote help, a version
// or a usage error and the caller should exit with code; otherwise ns holds the values.
func (p *Parser) Parse(args []string, s cli.Stdio) (ns *Namespace, code int, done bool) {
	ctx := newCtx(s)
	ns, extras, err := p.parseKnownArgs2(ctx, args, nil)
	if err != nil {
		return nil, err.(*exitStatus).code, true
	}
	if len(extras) > 0 {
		return nil, p.Error(ctx.err, "unrecognized arguments: "+strings.Join(extras, " ")), true
	}
	return ns, 0, false
}

// ParseKnown is parse_known_args(args): like Parse, but leftover strings are returned.
func (p *Parser) ParseKnown(args []string, s cli.Stdio) (ns *Namespace, extras []string, code int, done bool) {
	ns, extras, err := p.parseKnownArgs2(newCtx(s), args, nil)
	if err != nil {
		return nil, nil, err.(*exitStatus).code, true
	}
	return ns, extras, 0, false
}

// parseKnownArgs2 fills the defaults, parses, and turns an argument error into this
// parser's usage error (exit_on_error=True). The error is always an *exitStatus.
func (p *Parser) parseKnownArgs2(ctx *runCtx, args []string, ns *Namespace) (*Namespace, []string, error) {
	args = append([]string(nil), args...)
	if ns == nil {
		ns = NewNamespace()
	}
	for _, a := range p.actions {
		if a.Dest != Suppress && !ns.Has(a.Dest) && !isSuppress(a.Default) {
			ns.setDefault(a.Dest, a.Default)
		}
	}
	for _, k := range p.defaultKeys {
		if !ns.Has(k) {
			ns.setDefault(k, p.defaults[k])
		}
	}
	extras, err := p.parseKnownArgs(ctx, args, ns)
	if err != nil {
		if st, ok := err.(*exitStatus); ok {
			return nil, nil, st
		}
		return nil, nil, &exitStatus{code: p.Error(ctx.err, err.Error())}
	}
	if ns.unrecognized != nil {
		extras = append(extras, ns.unrecognized...)
		ns.unrecognized = nil
	}
	return ns, extras, nil
}

// parseState is the local state of one _parse_known_args call: the classified strings,
// the actions seen so far and the strings no action consumed.
type parseState struct {
	p              *Parser
	ctx            *runCtx
	ns             *Namespace
	argStrings     []string
	optionIdx      map[int][]optTuple // option_string_indices
	pattern        string             // arg_strings_pattern: 'O', 'A' and '-' per string
	conflicts      map[*Action][]*Action
	seen           map[*Action]bool
	seenNonDefault map[*Action]bool
	positionals    []*Action // the positionals not consumed yet
	extras         []string
}

// parseKnownArgs is _parse_known_args (intermixed=False, no fromfile_prefix_chars).
func (p *Parser) parseKnownArgs(ctx *runCtx, argStrings []string, ns *Namespace) ([]string, error) {
	st := &parseState{
		p: p, ctx: ctx, ns: ns, argStrings: argStrings,
		conflicts:      p.mutexConflicts(),
		seen:           map[*Action]bool{},
		seenNonDefault: map[*Action]bool{},
		positionals:    p.positionalActions(),
	}
	st.optionIdx, st.pattern = p.classifyArgs(argStrings)
	if err := st.consumeAll(); err != nil {
		return nil, err
	}
	if err := st.checkRequired(); err != nil {
		return nil, err
	}
	return st.extras, nil
}

// mutexConflicts maps each mutually exclusive action to the other actions of its groups
// (action_conflicts).
func (p *Parser) mutexConflicts() map[*Action][]*Action {
	conflicts := map[*Action][]*Action{}
	for _, mg := range p.mutexGroups {
		for i, a := range mg.actions {
			conflicts[a] = append(conflicts[a], mg.actions[:i]...)
			conflicts[a] = append(conflicts[a], mg.actions[i+1:]...)
		}
	}
	return conflicts
}

// consumeAll is the main loop of _parse_known_args: alternate positionals and optionals
// up to the last option string, then consume the trailing positionals.
func (st *parseState) consumeAll() error {
	maxOption := -1
	for i := range st.optionIdx {
		if i > maxOption {
			maxOption = i
		}
	}
	start := 0
	for start <= maxOption {
		next := start
		for next <= maxOption {
			if _, ok := st.optionIdx[next]; ok {
				break
			}
			next++
		}
		if start != next {
			end, err := st.consumePositionals(start)
			if err != nil {
				return err
			}
			if end > start {
				start = end
				continue
			}
			start = end
		}
		if _, ok := st.optionIdx[start]; !ok {
			st.extras = append(st.extras, st.argStrings[start:next]...)
			start = next
		}
		var err error
		if start, err = st.consumeOptional(start); err != nil {
			return err
		}
	}
	stop, err := st.consumePositionals(start)
	if err != nil {
		return err
	}
	st.extras = append(st.extras, st.argStrings[stop:]...)
	return nil
}

// takeAction is take_action: convert the strings, reject a mutually exclusive conflict
// and run the action unless its value is SUPPRESS.
func (st *parseState) takeAction(a *Action, strs []string, optionString string) error {
	st.seen[a] = true
	values, suppressed, err := st.p.getValues(a, strs)
	if err != nil {
		return err
	}
	if a.isOptional() || len(strs) > 0 {
		st.seenNonDefault[a] = true
		for _, c := range st.conflicts[a] {
			if st.seenNonDefault[c] {
				name, _ := actionName(c)
				return newArgError(a, "not allowed with argument "+name)
			}
		}
	}
	if suppressed {
		return nil
	}
	return st.p.callAction(st.ctx, a, st.ns, values, optionString)
}

// takenAction is one queued take_action call of consume_optional (action_tuples).
type takenAction struct {
	a    *Action
	args []string
	opt  string
}

// consumeOptional is consume_optional: match the option at start (and any single-dash
// flags chained into its explicit argument) and return the index after its strings.
func (st *parseState) consumeOptional(start int) (int, error) {
	tuples := st.optionIdx[start]
	if len(tuples) > 1 {
		return 0, ambiguousOptionError(st.argStrings[start], tuples)
	}
	t := tuples[0]
	var todo []takenAction
	stop := 0
	for {
		if t.action == nil {
			st.extras = append(st.extras, st.argStrings[start])
			return start + 1, nil
		}
		if !t.hasArg {
			s := start + 1
			count, err := st.p.matchArgument(t.action, st.pattern[s:])
			if err != nil {
				return 0, err
			}
			stop = s + count
			todo = append(todo, takenAction{t.action, st.argStrings[s:stop], t.option})
			break
		}
		count, err := st.p.matchArgument(t.action, "A")
		if err != nil {
			return 0, err
		}
		if count == 0 && t.option[1] != '-' && t.arg != "" {
			// a single-dash flag taking no value: the rest of the string is more flags
			if (t.hasSep && t.sep != "") || t.arg[0] == '-' {
				return 0, ignoredArgError(t)
			}
			todo = append(todo, takenAction{t.action, nil, t.option})
			if next, ok := st.p.chainedShortFlag(t.arg); ok {
				t = next
				continue
			}
			st.extras = append(st.extras, "-"+t.arg)
			stop = start + 1
			break
		}
		if count == 1 {
			stop = start + 1
			todo = append(todo, takenAction{t.action, []string{t.arg}, t.option})
			break
		}
		return 0, ignoredArgError(t)
	}
	for _, x := range todo {
		if err := st.takeAction(x.a, x.args, x.opt); err != nil {
			return 0, err
		}
	}
	return stop, nil
}

// chainedShortFlag reads the explicit argument of a single-dash flag that takes no value
// as the next flag ("-xyz" is -x then "-yz"); ok is false when "-<first char>" is not an
// option string, and the whole "-"+arg then becomes an extra.
func (p *Parser) chainedShortFlag(arg string) (optTuple, bool) {
	first, rest := splitFirstRune(arg)
	t := optTuple{option: "-" + first}
	a, ok := p.optIndex[t.option]
	if !ok {
		return optTuple{}, false
	}
	t.action = a
	switch {
	case rest == "":
	case rest[0] == '=':
		t.sep, t.hasSep, t.arg, t.hasArg = "=", true, rest[1:], true
	default:
		t.sep, t.hasSep, t.arg, t.hasArg = "", true, rest, true
	}
	return t, true
}

// ambiguousOptionError is the "ambiguous option" error of consume_optional.
func ambiguousOptionError(arg string, tuples []optTuple) error {
	opts := make([]string, len(tuples))
	for i, t := range tuples {
		opts[i] = t.option
	}
	return newArgError(nil, "ambiguous option: "+arg+" could match "+strings.Join(opts, ", "))
}

// ignoredArgError is the "ignored explicit argument" error of consume_optional.
func ignoredArgError(t optTuple) error {
	return newArgError(t.action, "ignored explicit argument "+pycompat.Repr(t.arg))
}

// consumePositionals is consume_positionals: match as many remaining positionals as the
// pattern from start allows, drop the "--" they swallowed, and take each one.
func (st *parseState) consumePositionals(start int) (int, error) {
	counts := st.p.matchArgumentsPartial(st.positionals, st.pattern[start:])
	for i, n := range counts {
		a := st.positionals[i]
		strs := append([]string(nil), st.argStrings[start:start+n]...)
		if a.nargs == nargsPar {
			if st.pattern[start] == '-' {
				strs = strs[1:]
			}
		} else if strings.IndexByte(st.pattern[start:start+n], '-') >= 0 {
			strs = removeFirst(strs, "--")
		}
		start += n
		if err := st.takeAction(a, strs, ""); err != nil {
			return 0, err
		}
	}
	st.positionals = st.positionals[len(counts):]
	return start, nil
}

// checkRequired is the tail of _parse_known_args: convert string defaults still in
// place, then report missing required arguments and unsatisfied required groups.
func (st *parseState) checkRequired() error {
	var required []string
	for _, a := range st.p.actions {
		if st.seen[a] {
			continue
		}
		if a.Required {
			name, _ := actionName(a)
			required = append(required, name)
			continue
		}
		// convert a string default that is still in place (it may fail, like Python)
		if d, ok := a.Default.(string); ok && st.ns.defaulted[a.Dest] && st.ns.Get(a.Dest) == any(d) {
			v, err := st.p.getValue(a, d)
			if err != nil {
				return err
			}
			st.ns.Set(a.Dest, v)
		}
	}
	if len(required) > 0 {
		return newArgError(nil, "the following arguments are required: "+strings.Join(required, ", "))
	}
	for _, mg := range st.p.mutexGroups {
		if !mg.Required || anySeen(mg.actions, st.seenNonDefault) {
			continue
		}
		var names []string
		for _, a := range mg.actions {
			if a.Help != Suppress {
				name, _ := actionName(a)
				names = append(names, name)
			}
		}
		return newArgError(nil, "one of the arguments "+strings.Join(names, " ")+" is required")
	}
	return nil
}

func anySeen(actions []*Action, seen map[*Action]bool) bool {
	for _, a := range actions {
		if seen[a] {
			return true
		}
	}
	return false
}

func removeFirst(strs []string, s string) []string {
	for i, x := range strs {
		if x == s {
			return append(strs[:i], strs[i+1:]...)
		}
	}
	return strs
}

func (p *Parser) positionalActions() []*Action {
	var out []*Action
	for _, a := range p.actions {
		if !a.isOptional() {
			out = append(out, a)
		}
	}
	return out
}

// call is _SubParsersAction.__call__: run the command's parser on the remaining strings
// and copy its namespace over the caller's.
func (s *Subparsers) call(ctx *runCtx, ns *Namespace, values []any) error {
	name := pycompat.Str(values[0])
	rest := make([]string, 0, len(values)-1)
	for _, v := range values[1:] {
		rest = append(rest, pycompat.Str(v))
	}
	if s.action.Dest != Suppress {
		ns.Set(s.action.Dest, name)
	}
	sub := s.parsers[name]
	subns, extras, err := sub.parseKnownArgs2(ctx, rest, nil)
	if err != nil {
		return err
	}
	for _, k := range subns.Keys() {
		ns.Set(k, subns.Get(k))
	}
	ns.unrecognized = append(ns.unrecognized, extras...)
	return nil
}
