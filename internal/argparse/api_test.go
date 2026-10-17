package argparse

import (
	"bytes"
	"errors"
	"testing"

	"github.com/angeles725/niagara-go/internal/cli"
)

// The API paths the fixture matrix does not drive: parse_known_args, error(), the
// namespace getters, Custom action errors and definition-time panics. Expected strings
// were taken from Python 3.15 argparse.

func smallParser() *Parser {
	p := New(Config{Prog: "t"})
	p.Add(Arg{Type: Int, Default: 1}, "--n")
	p.Add(Arg{Nargs: ZeroOrMore}, "items")
	p.Add(Arg{Action: Custom, Nargs: NoArgs, Call: func(*Call) error { return errors.New("boom") }}, "--x")
	return p
}

func TestParseKnownKeepsExtras(t *testing.T) {
	t.Setenv("COLUMNS", "100")
	ns, extras, _, done := smallParser().ParseKnown([]string{"a", "--zz", "b", "--n", "3"}, cli.Stdio{})
	if done {
		t.Fatal("parse_known_args exited")
	}
	if got := ns.Repr(); got != "{'items': ['a'], 'n': 3, 'x': None}" {
		t.Errorf("namespace %s", got)
	}
	if len(extras) != 2 || extras[0] != "--zz" || extras[1] != "b" {
		t.Errorf("extras %q", extras)
	}
	if ns.Int("n") != 3 || ns.Strings("items")[0] != "a" || ns.Has("zz") || ns.Bool("x") {
		t.Errorf("getters: n=%d items=%q", ns.Int("n"), ns.Strings("items"))
	}
}

func TestCustomActionErrorAndParserError(t *testing.T) {
	t.Setenv("COLUMNS", "100")
	const usage = "usage: t [-h] [--n N] [--x] [items ...]\n"
	var errb bytes.Buffer
	_, code, done := smallParser().Parse([]string{"--x"}, cli.Stdio{Err: &errb})
	if !done || code != 2 || errb.String() != usage+"t: error: argument --x: boom\n" {
		t.Errorf("custom error: done=%v code=%d %q", done, code, errb.String())
	}
	errb.Reset()
	if code := smallParser().Error(&errb, "custom"); code != 2 || errb.String() != usage+"t: error: custom\n" {
		t.Errorf("Error: code=%d %q", code, errb.String())
	}
	p := New(Config{Prog: "t", ErrorExit: 3})
	if code := p.Error(&errb, "x"); code != 3 {
		t.Errorf("ErrorExit: %d", code)
	}
}

func TestNamespaceGetters(t *testing.T) {
	ns := NewNamespace()
	ns.Set("s", "v")
	ns.Set("f", 2.5)
	ns.Set("l", []any{"a", 1})
	ns.Set("none", nil)
	if ns.String("s") != "v" || ns.String("none") != "" || ns.Float("f") != 2.5 || ns.Int("f") != 2 {
		t.Error("scalar getters")
	}
	if l := ns.Strings("l"); len(l) != 2 || l[1] != "1" {
		t.Errorf("Strings %q", l)
	}
	if ns.Strings("none") != nil || !ns.Has("none") || ns.Bool("none") {
		t.Error("None handling")
	}
	ns.Delete("s")
	if ns.Has("s") {
		t.Error("Delete")
	}
}

func TestDefinitionErrorsPanic(t *testing.T) {
	cases := map[string]func(){
		"conflict":        func() { New(Config{}).Add(Arg{}, "-h") },
		"mutex required":  func() { New(Config{}).AddMutuallyExclusiveGroup(false).Add(Arg{Required: true}, "--a") },
		"bad help":        func() { New(Config{}).Add(Arg{Help: "100% sure"}, "--a") },
		"metavar tuple":   func() { New(Config{}).Add(Arg{Nargs: 2, MetavarTuple: []string{"A"}}, "--a") },
		"positional flag": func() { New(Config{}).Add(Arg{Action: StoreTrue}, "pos") },
		"two subparsers": func() {
			p := New(Config{})
			p.AddSubparsers(SubparsersConfig{})
			p.AddSubparsers(SubparsersConfig{})
		},
	}
	for name, f := range cases {
		t.Run(name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Error("no panic")
				}
			}()
			f()
		})
	}
}
