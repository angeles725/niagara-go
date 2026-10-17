package argparse

import (
	"bytes"
	"math"
	"strings"
	"testing"

	"github.com/angeles725/niagara-go/internal/cli"
	"github.com/angeles725/niagara-go/internal/pycompat"
)

// Review findings on the first argparse port; every expectation was printed by Python 3.15.

// int() has no range: -2**63 and 10**20 parse; type=int keeps the exact value.
func TestIntTypeKeepsPythonRange(t *testing.T) {
	t.Setenv("COLUMNS", "100")
	if n, err := pycompat.ParseInt("-9223372036854775808"); err != nil || n != math.MinInt64 {
		t.Errorf("pycompat.ParseInt(-2**63) = %d, %v", n, err)
	}
	p := New(Config{Prog: "t"})
	p.Add(Arg{Type: Int}, "-n")
	for arg, want := range map[string]string{"-9223372036854775808": "{'n': -9223372036854775808}",
		"99999999999999999999": "{'n': 99999999999999999999}"} {
		ns, _, done := p.Parse([]string{"-n", arg}, cli.Stdio{})
		if done || ns.Repr() != want {
			t.Errorf("-n %s: done=%v ns=%v, want %s", arg, done, ns, want)
		}
	}
}

// CPython's _handle_long_word has the same `space_left > 0` guard: a line that is exactly
// full before a long word is emitted as is and the word is broken on the next lines.
func TestLongWordAfterFullLineMatchesTextwrap(t *testing.T) {
	cases := []struct {
		text  string
		width int
		want  []string
	}{
		{"abcde " + strings.Repeat("x", 20), 6, []string{"abcde", "xxxxxx", "xxxxxx", "xxxxxx", "xx"}},
		{"ab cd " + strings.Repeat("y", 9), 5, []string{"ab cd", "yyyyy", "yyyy"}},
		{"abcdef-ghijkl", 6, []string{"abcdef", "-", "ghijkl"}},
		{"a " + strings.Repeat("b", 10), 2, []string{"a", "bb", "bb", "bb", "bb", "bb"}},
	}
	for _, c := range cases {
		if got := wrapText(c.text, c.width, ""); strings.Join(got, "|") != strings.Join(c.want, "|") {
			t.Errorf("wrap(%q, %d) = %q, want %q", c.text, c.width, got, c.want)
		}
	}
}

// set_defaults(b=1, a=2, c=3) keeps keyword order (Python dict order).
func TestSetDefaultsKeepsCallOrder(t *testing.T) {
	p := New(Config{Prog: "t"})
	p.SetDefaults("b", 1, "a", 2, "c", 3)
	if got := strings.Join(p.defaultKeys, ","); got != "b,a,c" {
		t.Errorf("default order %s, want b,a,c", got)
	}
	ns, _, _ := p.Parse(nil, cli.Stdio{})
	if ns.Repr() != "{'a': 2, 'b': 1, 'c': 3}" {
		t.Errorf("namespace %s", ns.Repr())
	}
}

// _get_optional_kwargs: a single-dash long option outranks a short one wherever it is, and
// "short" means at most two code points ('-é' is short).
func TestInferDestLikeGetOptionalKwargs(t *testing.T) {
	for names, want := range map[string]string{"-x -foo": "foo", "-é -bar": "bar", "-yy -z": "yy",
		"-x --long-name -foo": "long_name", "-é": "é"} {
		if got := inferDest(strings.Fields(names)); got != want {
			t.Errorf("dest of %s = %s, want %s", names, got, want)
		}
	}
}

// action='extend' with nargs=None extends with the characters of the one string.
func TestExtendWithOneStringExtendsByCharacters(t *testing.T) {
	p := New(Config{Prog: "t"})
	p.Add(Arg{Action: Extend}, "--ex")
	ns, _, done := p.Parse([]string{"--ex", "abc", "--ex", "dé"}, cli.Stdio{})
	if done || ns.Repr() != "{'ex': ['a', 'b', 'c', 'd', 'é']}" {
		t.Errorf("extend: done=%v %v", done, ns)
	}
}

// A malformed fixture (exit null without a namespace) fails its case instead of panicking.
func TestCheckCaseReportsMissingNamespace(t *testing.T) {
	var problems []string
	checkResult(fixtureCase{}, "", "", NewNamespace(), 0, false, func(f string, a ...any) {
		problems = append(problems, f)
	})
	if len(problems) != 1 {
		t.Errorf("problems %q", problems)
	}
}

func TestErrorExitMapping(t *testing.T) {
	t.Setenv("COLUMNS", "100")
	for cfg, want := range map[int]int{0: 2, 3: 3, 1: 1} {
		p := New(Config{Prog: "t", ErrorExit: cfg})
		p.Add(Arg{Type: Int}, "-n")
		var errb bytes.Buffer
		if code := p.Error(&errb, "m"); code != want {
			t.Errorf("ErrorExit %d: Error() = %d", cfg, code)
		}
		for _, argv := range [][]string{{"-n", "x"}, {"extra"}, {"-n"}} {
			if _, code, done := p.Parse(argv, cli.Stdio{Err: &errb}); !done || code != want {
				t.Errorf("ErrorExit %d, %q: done=%v code=%d", cfg, argv, done, code)
			}
		}
		if _, code, _ := p.Parse([]string{"-h"}, cli.Stdio{Out: &errb}); code != 0 {
			t.Errorf("help exit %d", code)
		}
	}
}

func TestNormalizeSpaceAndPadRight(t *testing.T) {
	if got := normalizeSpace(" \t a \n\r\f\v b\u00a0 \u2003"); got != "a b" {
		t.Errorf("normalizeSpace = %q", got)
	}
	if got := normalizeSpace("a\u00a0\u00a0b"); got != "a\u00a0\u00a0b" {
		t.Errorf("normalizeSpace kept NBSP? %q", got)
	}
	for _, c := range []struct {
		s     string
		width int
		want  string
	}{{"ab", 4, "ab  "}, {"ñé", 3, "ñé "}, {"abc", 2, "abc"}, {"", 0, ""}} {
		if got := padRight(c.s, c.width); got != c.want {
			t.Errorf("padRight(%q, %d) = %q", c.s, c.width, got)
		}
	}
}

func TestNamespaceAccessorsAllKinds(t *testing.T) {
	ns := NewNamespace()
	for k, v := range map[string]any{"t": true, "f": false, "i": 7, "x": 1.5, "s": "", "l": []string{"a"},
		"e": []any{}, "n": nil} {
		ns.Set(k, v)
	}
	if ns.Int("t") != 1 || ns.Int("f") != 0 || ns.Int("x") != 1 || ns.Int("s") != 0 || ns.Int("missing") != 0 {
		t.Error("Int")
	}
	if ns.Float("i") != 7 || ns.Float("s") != 0 {
		t.Error("Float")
	}
	if ns.String("i") != "7" || ns.String("t") != "True" || ns.String("l") != "['a']" {
		t.Errorf("String %q %q %q", ns.String("i"), ns.String("t"), ns.String("l"))
	}
	if ns.Bool("s") || ns.Bool("e") || !ns.Bool("l") || !ns.Bool("x") || ns.Bool("n") || ns.Bool("missing") {
		t.Error("Bool")
	}
	if l := ns.Strings("i"); len(l) != 1 || l[0] != "7" {
		t.Errorf("Strings scalar %q", l)
	}
	if got := strings.Join(ns.Keys(), ","); got != "e,f,i,l,n,s,t,x" {
		t.Errorf("Keys %s", got)
	}
	if ns.Get("missing") != nil || ns.Has("missing") {
		t.Error("missing")
	}
}
