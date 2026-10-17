package argparse

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"testing"

	"github.com/angeles725/niagara-go/internal/pycompat"
)

// This file builds a Parser from the declarative spec that gen_fixtures.py also builds a
// Python ArgumentParser from (the format is documented at the top of gen_fixtures.py).

type spec = map[string]any

var testTypes = map[string]*Type{
	"int": Int, "float": Float, "str": String,
	"positive_int": {Name: "positive_int", Convert: func(s string) (any, error) {
		n, err := pycompat.ParseInt(s)
		if err != nil {
			return nil, NewTypeError("invalid int value: %s", pycompat.Repr(s))
		}
		if n < 1 {
			return nil, NewTypeError("must be at least 1, not %d", n)
		}
		return int(n), nil
	}},
	"nav_positive_int": {Name: "nav_positive_int", Convert: func(s string) (any, error) {
		n, err := pycompat.ParseInt(s)
		if err != nil || n < 1 {
			return nil, NewTypeError("must be a positive integer, got %s", pycompat.Repr(s))
		}
		return int(n), nil
	}},
	"checked": {Name: "checked", Convert: func(s string) (any, error) {
		if strings.HasPrefix(s, "bad") {
			return nil, fmt.Errorf("bad value")
		}
		return strings.ToUpper(s), nil
	}},
}

var testFormatters = map[string]Formatter{
	"HelpFormatter": HelpFormatter, "RawDescriptionHelpFormatter": RawDescriptionHelpFormatter,
	"RawTextHelpFormatter": RawTextHelpFormatter, "ArgumentDefaultsHelpFormatter": ArgumentDefaultsHelpFormatter,
}

var testActions = map[string]ActionKind{
	"store": Store, "store_const": StoreConst, "store_true": StoreTrue, "store_false": StoreFalse,
	"append": Append, "append_const": AppendConst, "count": Count, "extend": Extend,
	"help": Help, "version": Version,
}

// specValue converts a JSON value: integral numbers to int, "$SUPPRESS" to Suppress.
func specValue(v any) any {
	switch x := v.(type) {
	case json.Number:
		if i, err := x.Int64(); err == nil && !strings.ContainsAny(x.String(), ".eE") {
			return int(i)
		}
		f, _ := x.Float64()
		return f
	case string:
		if x == "$SUPPRESS" {
			return Suppress
		}
		return x
	case []any:
		out := make([]any, len(x))
		for i, e := range x {
			out[i] = specValue(e)
		}
		return out
	}
	return v
}

func str(m spec, key string) string {
	s, _ := specValue(m[key]).(string)
	return s
}

func specConfig(t *testing.T, kw spec, parents map[string]*Parser) Config {
	c := Config{Prog: str(kw, "prog"), Usage: str(kw, "usage"), Description: str(kw, "description"),
		Epilog: str(kw, "epilog")}
	if f, ok := kw["formatter_class"]; ok {
		c.Formatter = testFormatters[f.(string)]
	}
	for _, name := range asList(kw["parents"]) {
		c.Parents = append(c.Parents, parents[name.(string)])
	}
	if v, ok := kw["add_help"]; ok && !v.(bool) {
		c.NoHelp = true
	}
	if v, ok := kw["allow_abbrev"]; ok && !v.(bool) {
		c.NoAbbrev = true
	}
	if v, ok := kw["_error_exit"]; ok {
		c.ErrorExit = specValue(v).(int)
	}
	for k := range kw {
		switch k {
		case "prog", "usage", "description", "epilog", "formatter_class", "parents", "add_help",
			"allow_abbrev", "_error_exit", "help", "aliases":
		default:
			t.Fatalf("spec: unsupported parser kwarg %q", k)
		}
	}
	return c
}

func sortedKeys(m spec) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func asList(v any) []any {
	l, _ := v.([]any)
	return l
}

func specArg(t *testing.T, kw spec) Arg {
	var a Arg
	tool := ""
	for k, raw := range kw {
		v := specValue(raw)
		switch k {
		case "action":
			if v == "tools_version" {
				a.Action, a.Nargs, a.Default = Custom, NoArgs, Suppress
				continue
			}
			kind, ok := testActions[v.(string)]
			if !ok {
				t.Fatalf("spec: unknown action %v", v)
			}
			a.Action = kind
		case "nargs":
			switch v {
			case "?":
				a.Nargs = Optional
			case "*":
				a.Nargs = ZeroOrMore
			case "+":
				a.Nargs = OneOrMore
			default:
				a.Nargs = v.(int)
			}
		case "const":
			a.Const = v
		case "default":
			a.Default = v
		case "type":
			a.Type = testTypes[v.(string)]
		case "choices":
			a.Choices = v.([]any)
		case "required":
			a.Required = v.(bool)
		case "help":
			a.Help = v.(string)
		case "metavar":
			if l, ok := v.([]any); ok {
				for _, e := range l {
					a.MetavarTuple = append(a.MetavarTuple, e.(string))
				}
			} else {
				a.Metavar = v.(string)
			}
		case "dest":
			a.Dest = v.(string)
		case "version":
			a.Version = v.(string)
		case "tool":
			tool = v.(string)
		default:
			t.Fatalf("spec: unsupported add_argument kwarg %q", k)
		}
	}
	if a.Action == Custom {
		if a.Help == "" {
			a.Help = "print the tools version and exit"
		}
		a.Call = func(c *Call) error {
			name := tool
			if name == "" {
				name = strings.TrimSuffix(c.Parser.Prog(), ".py")
			}
			fmt.Fprintf(c.Out, "%s 0.42.0\n", name)
			return &ExitError{Code: 0}
		}
	}
	return a
}

type specContainer interface {
	Add(a Arg, names ...string) *Action
}

func specOps(t *testing.T, c specContainer, ops []any, parents map[string]*Parser) {
	for _, raw := range ops {
		op := raw.(spec)
		switch op["op"] {
		case "arg":
			var names []string
			for _, n := range asList(op["args"]) {
				names = append(names, n.(string))
			}
			kw, _ := op["kw"].(spec)
			c.Add(specArg(t, kw), names...)
		case "mutex":
			type mutexer interface {
				AddMutuallyExclusiveGroup(required bool) *MutexGroup
			}
			specOps(t, c.(mutexer).AddMutuallyExclusiveGroup(op["required"].(bool)), asList(op["ops"]), parents)
		case "group":
			title, _ := op["title"].(string)
			desc, _ := op["description"].(string)
			specOps(t, c.(*Parser).AddArgumentGroup(title, desc), asList(op["ops"]), parents)
		case "defaults":
			kw := op["kw"].(spec)
			var pairs []any
			for _, k := range sortedKeys(kw) {
				pairs = append(pairs, k, specValue(kw[k]))
			}
			c.(*Parser).SetDefaults(pairs...)
		case "set":
			p := c.(*Parser)
			switch op["attr"] {
			case "epilog":
				p.SetEpilog(op["value"].(string))
			case "description":
				p.SetDescription(op["value"].(string))
			case "usage":
				p.SetUsage(op["value"].(string))
			}
		case "subparsers":
			specSubparsers(t, c.(*Parser), op, parents)
		default:
			t.Fatalf("spec: unknown op %v", op["op"])
		}
	}
}

func specSubparsers(t *testing.T, p *Parser, op spec, parents map[string]*Parser) {
	kw, _ := op["kw"].(spec)
	sc := SubparsersConfig{Title: str(kw, "title"), Description: str(kw, "description"),
		Dest: str(kw, "dest"), Help: str(kw, "help"), Metavar: str(kw, "metavar"), Prog: str(kw, "prog")}
	if v, ok := kw["required"]; ok {
		sc.Required = v.(bool)
	}
	sub := p.AddSubparsers(sc)
	if v, ok := op["required"]; ok {
		sub.SetRequired(v.(bool))
	}
	for _, raw := range asList(op["parsers"]) {
		child := raw.(spec)
		ckw, _ := child["kw"].(spec)
		cfg := SubparserConfig{Config: specConfig(t, ckw, parents)}
		if h, ok := ckw["help"]; ok {
			cfg.Help, cfg.Listed = specValue(h).(string), true
		}
		for _, a := range asList(ckw["aliases"]) {
			cfg.Aliases = append(cfg.Aliases, a.(string))
		}
		sp := sub.AddParser(child["name"].(string), cfg)
		specOps(t, sp, asList(child["ops"]), parents)
	}
}

func specParser(t *testing.T, s spec, parents map[string]*Parser) *Parser {
	kw, _ := s["kw"].(spec)
	p := New(specConfig(t, kw, parents))
	specOps(t, p, asList(s["ops"]), parents)
	return p
}
