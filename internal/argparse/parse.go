package argparse

import (
	"io"
	"strings"

	"github.com/angeles725/niagara-go/internal/cli"
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

// optTuple is one interpretation of an option string: (action, option_string, sep,
// explicit_arg); hasSep and hasArg false stand for None.
type optTuple struct {
	action *Action
	option string
	sep    string
	hasSep bool
	arg    string
	hasArg bool
}

// parseKnownArgs is _parse_known_args (intermixed=False, no fromfile_prefix_chars).
func (p *Parser) parseKnownArgs(ctx *runCtx, argStrings []string, ns *Namespace) ([]string, error) {
	conflicts := map[*Action][]*Action{}
	for _, mg := range p.mutexGroups {
		for i, a := range mg.actions {
			conflicts[a] = append(conflicts[a], mg.actions[:i]...)
			conflicts[a] = append(conflicts[a], mg.actions[i+1:]...)
		}
	}

	optionIdx := map[int][]optTuple{}
	var pat strings.Builder
	for i := 0; i < len(argStrings); i++ {
		if argStrings[i] == "--" {
			pat.WriteByte('-')
			pat.WriteString(strings.Repeat("A", len(argStrings)-i-1))
			break
		}
		if t := p.parseOptional(argStrings[i]); t != nil {
			optionIdx[i] = t
			pat.WriteByte('O')
		} else {
			pat.WriteByte('A')
		}
	}
	pattern := pat.String()

	seen := map[*Action]bool{}
	seenNonDefault := map[*Action]bool{}
	takeAction := func(a *Action, strs []string, optionString string) error {
		seen[a] = true
		values, suppressed, err := p.getValues(a, strs)
		if err != nil {
			return err
		}
		if a.isOptional() || len(strs) > 0 {
			seenNonDefault[a] = true
			for _, c := range conflicts[a] {
				if seenNonDefault[c] {
					name, _ := actionName(c)
					return newArgError(a, "not allowed with argument "+name)
				}
			}
		}
		if suppressed {
			return nil
		}
		return p.callAction(ctx, a, ns, values, optionString)
	}

	var extras []string
	consumeOptional := func(start int) (int, error) {
		tuples := optionIdx[start]
		if len(tuples) > 1 {
			opts := make([]string, len(tuples))
			for i, t := range tuples {
				opts[i] = t.option
			}
			return 0, newArgError(nil, "ambiguous option: "+argStrings[start]+" could match "+strings.Join(opts, ", "))
		}
		t := tuples[0]
		type taken struct {
			a    *Action
			args []string
			opt  string
		}
		var todo []taken
		stop := 0
		for {
			if t.action == nil {
				extras = append(extras, argStrings[start])
				return start + 1, nil
			}
			if !t.hasArg {
				s := start + 1
				count, err := p.matchArgument(t.action, pattern[s:])
				if err != nil {
					return 0, err
				}
				stop = s + count
				todo = append(todo, taken{t.action, argStrings[s:stop], t.option})
				break
			}
			count, err := p.matchArgument(t.action, "A")
			if err != nil {
				return 0, err
			}
			if count == 0 && t.option[1] != '-' && t.arg != "" {
				// a single-dash flag taking no value: the rest of the string is more flags
				if (t.hasSep && t.sep != "") || t.arg[0] == '-' {
					return 0, newArgError(t.action, "ignored explicit argument "+pyRepr(t.arg))
				}
				todo = append(todo, taken{t.action, nil, t.option})
				first, rest := splitFirstRune(t.arg)
				t.option = "-" + first
				if a2, ok := p.optIndex[t.option]; ok {
					t.action = a2
					switch {
					case rest == "":
						t.sep, t.hasSep, t.arg, t.hasArg = "", false, "", false
					case rest[0] == '=':
						t.sep, t.hasSep, t.arg = "=", true, rest[1:]
					default:
						t.sep, t.hasSep, t.arg = "", true, rest
					}
					continue
				}
				extras = append(extras, "-"+t.arg)
				stop = start + 1
				break
			}
			if count == 1 {
				stop = start + 1
				todo = append(todo, taken{t.action, []string{t.arg}, t.option})
				break
			}
			return 0, newArgError(t.action, "ignored explicit argument "+pyRepr(t.arg))
		}
		for _, x := range todo {
			if err := takeAction(x.a, x.args, x.opt); err != nil {
				return 0, err
			}
		}
		return stop, nil
	}

	positionals := p.positionalActions()
	consumePositionals := func(start int) (int, error) {
		counts := p.matchArgumentsPartial(positionals, pattern[start:])
		for i, n := range counts {
			a := positionals[i]
			strs := append([]string(nil), argStrings[start:start+n]...)
			if a.nargs == nargsPar {
				if pattern[start] == '-' {
					strs = strs[1:]
				}
			} else if strings.IndexByte(pattern[start:start+n], '-') >= 0 {
				strs = removeFirst(strs, "--")
			}
			start += n
			if err := takeAction(a, strs, ""); err != nil {
				return 0, err
			}
		}
		positionals = positionals[len(counts):]
		return start, nil
	}

	maxOption := -1
	for i := range optionIdx {
		if i > maxOption {
			maxOption = i
		}
	}
	start := 0
	for start <= maxOption {
		next := start
		for next <= maxOption {
			if _, ok := optionIdx[next]; ok {
				break
			}
			next++
		}
		if start != next {
			end, err := consumePositionals(start)
			if err != nil {
				return nil, err
			}
			if end > start {
				start = end
				continue
			}
			start = end
		}
		if _, ok := optionIdx[start]; !ok {
			extras = append(extras, argStrings[start:next]...)
			start = next
		}
		var err error
		if start, err = consumeOptional(start); err != nil {
			return nil, err
		}
	}
	stop, err := consumePositionals(start)
	if err != nil {
		return nil, err
	}
	extras = append(extras, argStrings[stop:]...)

	var required []string
	for _, a := range p.actions {
		if seen[a] {
			continue
		}
		if a.Required {
			name, _ := actionName(a)
			required = append(required, name)
			continue
		}
		// convert a string default that is still in place (it may fail, like Python)
		if d, ok := a.Default.(string); ok && ns.defaulted[a.Dest] && ns.Get(a.Dest) == any(d) {
			v, err := p.getValue(a, d)
			if err != nil {
				return nil, err
			}
			ns.Set(a.Dest, v)
		}
	}
	if len(required) > 0 {
		return nil, newArgError(nil, "the following arguments are required: "+strings.Join(required, ", "))
	}
	for _, mg := range p.mutexGroups {
		if !mg.Required || anySeen(mg.actions, seenNonDefault) {
			continue
		}
		var names []string
		for _, a := range mg.actions {
			if a.Help != Suppress {
				name, _ := actionName(a)
				names = append(names, name)
			}
		}
		return nil, newArgError(nil, "one of the arguments "+strings.Join(names, " ")+" is required")
	}
	return extras, nil
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

func splitFirstRune(s string) (string, string) {
	for i := range s {
		if i > 0 {
			return s[:i], s[i:]
		}
	}
	return s, ""
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

// parseOptional is _parse_optional: nil when the string is a positional argument.
func (p *Parser) parseOptional(s string) []optTuple {
	if s == "" || s[0] != '-' {
		return nil
	}
	if a, ok := p.optIndex[s]; ok {
		return []optTuple{{action: a, option: s}}
	}
	if s == "-" {
		return nil
	}
	if opt, arg, found := strings.Cut(s, "="); found {
		if a, ok := p.optIndex[opt]; ok {
			return []optTuple{{action: a, option: opt, sep: "=", hasSep: true, arg: arg, hasArg: true}}
		}
	}
	if t := p.optionTuples(s); len(t) > 0 {
		return t
	}
	if negativeNumber(s) && !p.negOptions {
		return nil
	}
	if strings.Contains(s, " ") {
		return nil
	}
	return []optTuple{{option: s}}
}

// optionTuples is _get_option_tuples: the abbreviations and short-option-with-value
// readings of s.
func (p *Parser) optionTuples(s string) []optTuple {
	var out []optTuple
	prefix, arg, found := strings.Cut(s, "=")
	if s[1] == '-' {
		if p.cfg.NoAbbrev {
			return nil
		}
		for _, o := range p.optOrder {
			if strings.HasPrefix(o, prefix) {
				out = append(out, optTuple{action: p.optIndex[o], option: o, sep: "=", hasSep: found, arg: arg, hasArg: found})
			}
		}
		return out
	}
	first, rest := splitFirstRune(s[1:])
	short := "-" + first
	for _, o := range p.optOrder {
		if o == short {
			out = append(out, optTuple{action: p.optIndex[o], option: o, hasSep: true, arg: rest, hasArg: true})
		} else if !p.cfg.NoAbbrev && strings.HasPrefix(o, prefix) {
			out = append(out, optTuple{action: p.optIndex[o], option: o, sep: "=", hasSep: found, arg: arg, hasArg: found})
		}
	}
	return out
}

// call is _SubParsersAction.__call__: run the command's parser on the remaining strings
// and copy its namespace over the caller's.
func (s *Subparsers) call(ctx *runCtx, ns *Namespace, values []any) error {
	name := pyStr(values[0])
	rest := make([]string, 0, len(values)-1)
	for _, v := range values[1:] {
		rest = append(rest, pyStr(v))
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
