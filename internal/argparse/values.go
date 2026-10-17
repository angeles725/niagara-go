package argparse

import (
	"errors"
	"fmt"
	"math/big"
	"regexp"
	"strings"
	"sync"
)

// argError is argparse.ArgumentError: "argument NAME: message", or the bare message when
// no argument is named.
type argError struct {
	name string
	msg  string
}

func (e *argError) Error() string {
	if e.name == "" {
		return e.msg
	}
	return "argument " + e.name + ": " + e.msg
}

func newArgError(a *Action, msg string) error {
	name, _ := actionName(a)
	return &argError{name: name, msg: msg}
}

var (
	patternMu    sync.Mutex
	patternCache = map[string]*regexp.Regexp{}
)

func compiled(pattern string) *regexp.Regexp {
	patternMu.Lock()
	defer patternMu.Unlock()
	re, ok := patternCache[pattern]
	if !ok {
		re = regexp.MustCompile("^" + pattern)
		patternCache[pattern] = re
	}
	return re
}

// nargsPattern is _get_nargs_pattern over the O/A/- string of the command line.
func nargsPattern(a *Action) string {
	opt := a.isOptional()
	pick := func(o, pos string) string {
		if opt {
			return o
		}
		return pos
	}
	switch {
	case a.nargsZero:
		return pick(`([AO]{0})`, `((?:-*A){0}-*)`)
	case a.nargs == 0:
		return pick(`(A)`, `(-*A-*)`)
	case a.nargs == Optional:
		return pick(`(A?)`, `(-*A?-*)`)
	case a.nargs == ZeroOrMore:
		return pick(`(A*)`, `(-*[A\-]*)`)
	case a.nargs == OneOrMore:
		return pick(`(A+)`, `(-*A[A\-]*)`)
	case a.nargs == nargsPar:
		return pick(`(A[AO]*)`, `(-*A[\-AO]*)`)
	}
	return pick(fmt.Sprintf(`([AO]{%d})`, a.nargs), fmt.Sprintf(`((?:-*A){%d}-*)`, a.nargs))
}

// matchArgument is _match_argument: how many strings of pattern the action consumes.
func (p *Parser) matchArgument(a *Action, pattern string) (int, error) {
	m := compiled(nargsPattern(a)).FindStringSubmatchIndex(pattern)
	if m == nil {
		var msg string
		switch {
		case a.nargs == 0 && !a.nargsZero:
			msg = "expected one argument"
		case a.nargs == Optional:
			msg = "expected at most one argument"
		case a.nargs == OneOrMore:
			msg = "expected at least one argument"
		case a.nargs == 1:
			msg = "expected 1 argument"
		default:
			msg = fmt.Sprintf("expected %d arguments", a.nargs)
		}
		return 0, newArgError(a, msg)
	}
	return m[3] - m[2], nil
}

// matchArgumentsPartial is _match_arguments_partial: the longest prefix of actions that
// matches, with the counts of each.
func (p *Parser) matchArgumentsPartial(actions []*Action, pattern string) []int {
	for i := len(actions); i > 0; i-- {
		var b strings.Builder
		for _, a := range actions[:i] {
			b.WriteString(nargsPattern(a))
		}
		m := compiled(b.String()).FindStringSubmatchIndex(pattern)
		if m == nil {
			continue
		}
		result := make([]int, i)
		for k := range result {
			result[k] = m[2*k+3] - m[2*k+2]
		}
		if m[1] < len(pattern) && pattern[m[1]] == 'O' {
			for len(result) > 0 && result[len(result)-1] == 0 {
				result = result[:len(result)-1]
			}
		}
		return result
	}
	return nil
}

// getValues is _get_values; suppressed reports a SUPPRESS value (no action is taken).
func (p *Parser) getValues(a *Action, strs []string) (any, bool, error) {
	switch {
	case len(strs) == 0 && a.nargs == Optional:
		v := a.Default
		if a.isOptional() {
			v = a.constant
		}
		if isSuppress(v) {
			return v, true, nil
		}
		if s, ok := v.(string); ok {
			conv, err := p.getValue(a, s)
			return conv, false, err
		}
		return v, false, nil
	case len(strs) == 0 && a.nargs == ZeroOrMore && !a.isOptional():
		if a.Default != nil {
			return a.Default, isSuppress(a.Default), nil
		}
		return []any{}, false, nil
	case len(strs) == 1 && ((a.nargs == 0 && !a.nargsZero) || a.nargs == Optional):
		v, err := p.getValue(a, strs[0])
		if err == nil {
			err = p.checkValue(a, v)
		}
		return v, false, err
	}
	values := make([]any, len(strs))
	for i, s := range strs {
		v, err := p.getValue(a, s)
		if err != nil {
			return nil, false, err
		}
		values[i] = v
	}
	for i, v := range values {
		if a.nargs == nargsPar && i > 0 {
			break
		}
		if err := p.checkValue(a, v); err != nil {
			return nil, false, err
		}
	}
	return values, false, nil
}

// getValue is _get_value: run the type converter.
func (p *Parser) getValue(a *Action, s string) (any, error) {
	if a.typ == nil || a.typ.Convert == nil {
		return s, nil
	}
	v, err := a.typ.Convert(s)
	if err != nil {
		var te *TypeError
		if errors.As(err, &te) {
			return nil, newArgError(a, te.Msg)
		}
		return nil, newArgError(a, "invalid "+a.typ.Name+" value: "+pyRepr(s))
	}
	return v, nil
}

// checkValue is _check_value: the value must be one of the choices.
func (p *Parser) checkValue(a *Action, v any) error {
	choices := a.choiceValues()
	if choices == nil && a.sub == nil {
		return nil
	}
	allStr := true
	quoted := make([]string, len(choices))
	names := make([]string, len(choices))
	for i, c := range choices {
		if pyEqual(v, c) {
			return nil
		}
		_, ok := c.(string)
		allStr = allStr && ok
		names[i] = pyStr(c)
		quoted[i] = pyRepr(names[i])
	}
	value := pyRepr(pyStr(v))
	list := strings.Join(quoted, ", ")
	if s, ok := v.(string); ok && !p.cfg.NoSuggest && allStr {
		if best, found := closeMatch(s, names); found {
			return newArgError(a, "invalid choice: "+value+", maybe you meant "+pyRepr(best)+"? (choose from "+list+")")
		}
	}
	return newArgError(a, "invalid choice: "+value+" (choose from "+list+")")
}

// pyEqual is Python == for the scalar kinds choices hold (True == 1 == 1.0).
func pyEqual(x, y any) bool {
	num := func(v any) (float64, bool) {
		switch n := v.(type) {
		case int:
			return float64(n), true
		case float64:
			return n, true
		case bool:
			if n {
				return 1, true
			}
			return 0, true
		}
		return 0, false
	}
	if bx, ok := x.(*big.Int); ok {
		by, ok := y.(*big.Int)
		return ok && bx.Cmp(by) == 0
	}
	if a, ok := num(x); ok {
		b, ok := num(y)
		return ok && a == b
	}
	return x == y
}

func copyItems(v any) []any {
	switch x := v.(type) {
	case nil:
		return []any{}
	case []any:
		return append([]any{}, x...)
	case []string:
		out := make([]any, len(x))
		for i, s := range x {
			out[i] = s
		}
		return out
	}
	return []any{v}
}

// iterItems is list(values) for extend: a list's items, a string's characters. Python
// raises TypeError for any other scalar; it is appended as one item instead.
func iterItems(v any) []any {
	switch x := v.(type) {
	case []any:
		return x
	case string:
		var out []any
		for _, r := range x {
			out = append(out, string(r))
		}
		return out
	}
	return []any{v}
}

// callAction is Action.__call__.
func (p *Parser) callAction(ctx *runCtx, a *Action, ns *Namespace, values any, option string) error {
	switch a.kind {
	case Store:
		ns.Set(a.Dest, values)
	case StoreConst, StoreTrue, StoreFalse:
		ns.Set(a.Dest, a.constant)
	case Append:
		ns.Set(a.Dest, append(copyItems(ns.Get(a.Dest)), values))
	case AppendConst:
		ns.Set(a.Dest, append(copyItems(ns.Get(a.Dest)), a.constant))
	case Extend:
		ns.Set(a.Dest, append(copyItems(ns.Get(a.Dest)), iterItems(values)...))
	case Count:
		n, _ := ns.Get(a.Dest).(int)
		ns.Set(a.Dest, n+1)
	case Help:
		p.PrintHelp(ctx.out)
		return &exitStatus{code: 0}
	case Version:
		f := newFormatter(p)
		f.addText(a.version)
		writeMsg(ctx.out, f.formatHelp())
		return &exitStatus{code: 0}
	case parsers:
		return a.sub.call(ctx, ns, values.([]any))
	case Custom:
		err := a.call(&Call{Parser: p, Namespace: ns, Values: values, OptionString: option, Out: ctx.out, Err: ctx.err})
		var ex *ExitError
		if errors.As(err, &ex) {
			writeMsg(ctx.err, ex.Message)
			return &exitStatus{code: ex.Code}
		}
		if err != nil {
			return newArgError(a, err.Error())
		}
	}
	return nil
}
