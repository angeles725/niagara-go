package argparse

import (
	"bytes"
	"testing"

	"github.com/angeles725/niagara-go/internal/cli"
)

// Pins for _parse_known_args branches the fixtures do not reach; every expectation was
// printed by Python 3.15 argparse.
func TestParseKnownArgsEdgeBranches(t *testing.T) {
	cases := []struct {
		name    string
		build   func(p *Parser)
		args    []string
		wantNS  string
		wantErr string
	}{
		{
			name:  "suppressed positional default leaves no attribute",
			build: func(p *Parser) { p.Add(Arg{Nargs: Optional, Default: Suppress}, "x") },
			args:  nil, wantNS: "{}",
		},
		{
			name:  "explicit =value for a two-value option",
			build: func(p *Parser) { p.Add(Arg{Nargs: 2}, "-o") },
			args:  []string{"-o=x"}, wantErr: "usage: t [-h] [-o O O]\nt: error: argument -o: expected 2 arguments\n",
		},
		{
			name:  "attached short value for a two-value option",
			build: func(p *Parser) { p.Add(Arg{Nargs: 2}, "-o") },
			args:  []string{"-ox"}, wantErr: "usage: t [-h] [-o O O]\nt: error: argument -o: expected 2 arguments\n",
		},
		{
			name: "subcommand after a leading --",
			build: func(p *Parser) {
				p.AddSubparsers(SubparsersConfig{Dest: "cmd"}).AddParser("go", SubparserConfig{}).Add(Arg{}, "y")
			},
			args: []string{"--", "go", "1"}, wantNS: "{'cmd': 'go', 'y': '1'}",
		},
		{
			name:  "unconvertible string default is an argument error",
			build: func(p *Parser) { p.Add(Arg{Type: Int, Default: "abc"}, "-n") },
			args:  nil, wantErr: "usage: t [-h] [-n N]\nt: error: argument -n: invalid int value: 'abc'\n",
		},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			p := New(Config{Prog: "t"})
			tt.build(p)
			var errb bytes.Buffer
			ns, code, done := p.Parse(tt.args, cli.Stdio{Err: &errb})
			if tt.wantErr != "" {
				if !done || code != 2 || errb.String() != tt.wantErr {
					t.Fatalf("done=%v code=%d stderr=%q, want exit 2 with %q", done, code, errb.String(), tt.wantErr)
				}
				return
			}
			if done || ns.Repr() != tt.wantNS {
				t.Fatalf("done=%v code=%d stderr=%q ns=%v, want %s", done, code, errb.String(), ns, tt.wantNS)
			}
		})
	}
}
