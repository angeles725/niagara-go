package argparse

import (
	"bytes"
	"math"
	"testing"

	"github.com/angeles725/niagara-go/internal/cli"
)

// TestPositiveInt pins cliflags.positive_int: Python int() parsing, a floor of 1 that names
// the parsed value, and no upper bound (a huge value saturates to math.MaxInt).
func TestPositiveInt(t *testing.T) {
	tests := []struct {
		name, in string
		want     int
		msg      string
	}{
		{name: "one", in: "1", want: 1},
		{name: "python int syntax", in: " +1_0 ", want: 10},
		{name: "huge saturates", in: "99999999999999999999999", want: math.MaxInt},
		{name: "zero", in: "0", msg: "must be at least 1, not 0"},
		{name: "negative", in: "-2", msg: "must be at least 1, not -2"},
		{name: "huge negative names the value", in: "-99999999999999999999999",
			msg: "must be at least 1, not -99999999999999999999999"},
		{name: "not a number", in: "x", msg: "invalid int value: 'x'"},
		{name: "quote picks the repr", in: "it's", msg: `invalid int value: "it's"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v, err := PositiveInt.Convert(tt.in)
			if tt.msg != "" {
				if err == nil || err.Error() != tt.msg {
					t.Fatalf("Convert(%q) error = %v, want %q", tt.in, err, tt.msg)
				}
				return
			}
			if err != nil || v != tt.want {
				t.Fatalf("Convert(%q) = %v, %v; want %d", tt.in, v, err, tt.want)
			}
		})
	}
	if PositiveInt.Name != "positive_int" {
		t.Errorf("Name = %q", PositiveInt.Name)
	}
}

// TestOutputParent checks both output_parents copies: the main parser's (False/None
// defaults) and the subcommand's (SUPPRESS, so it never resets the main parser's values).
func TestOutputParent(t *testing.T) {
	t.Setenv("COLUMNS", "100")
	p := New(Config{Prog: "t", Parents: []*Parser{OutputParent(false)}})
	sub := p.AddSubparsers(SubparsersConfig{Dest: "cmd"})
	sub.AddParser("go", SubparserConfig{Config: Config{Parents: []*Parser{OutputParent(true)}}})

	ns, code, done := p.Parse([]string{"--json", "-n", "3", "go"}, cli.Stdio{})
	if done {
		t.Fatalf("parse exited %d", code)
	}
	if got := ns.Repr(); got != "{'cmd': 'go', 'csv': False, 'json': True, 'limit': 3}" {
		t.Errorf("main flags kept: %s", got)
	}
	ns, _, _ = p.Parse([]string{"go", "--csv", "--limit", "2"}, cli.Stdio{})
	if got := ns.Repr(); got != "{'cmd': 'go', 'csv': True, 'json': False, 'limit': 2}" {
		t.Errorf("subcommand flags: %s", got)
	}

	var stderr bytes.Buffer
	_, code, done = p.Parse([]string{"-n", "0"}, cli.Stdio{Err: &stderr})
	want := "usage: t [-h] [--json] [--csv] [--limit N] {go} ...\n" +
		"t: error: argument --limit/-n: must be at least 1, not 0\n"
	if !done || code != 2 || stderr.String() != want {
		t.Errorf("limit 0: code %d done %v stderr %q", code, done, stderr.String())
	}
}
