package obixnav

import (
	"github.com/angeles725/niagara-go/internal/argparse"
	"github.com/angeles725/niagara-go/internal/cli"
)

// Defaults of the Python tool: the SSH tunnel to the PANCCADIA JACE and the pass file of the
// session that wrote it.
const (
	DefaultBase     = "https://127.0.0.1:18443/obix"
	DefaultPassFile = "/tmp/claude-1000/-home-cristian-niagara-research/" +
		"ab9d38a9-7835-4223-82e2-2e733a5e573a/scratchpad/.obix_pass"
)

// Main is the obix-nav command line (Python argparse parity).
func Main(args []string, s cli.Stdio) int {
	p := argparse.New(argparse.Config{Prog: "obix-nav.py"})
	p.Add(argparse.Arg{Default: DefaultBase}, "--base")
	p.Add(argparse.Arg{Default: DefaultPassFile}, "--pass-file")
	sub := p.AddSubparsers(argparse.SubparsersConfig{Dest: "cmd", Required: true})
	m := sub.AddParser("map", argparse.SubparserConfig{})
	m.Add(argparse.Arg{Type: argparse.Int}, "--room")
	m.Add(argparse.Arg{Action: argparse.StoreTrue}, "--json")
	sub.AddParser("cond", argparse.SubparserConfig{})
	sub.AddParser("wiring", argparse.SubparserConfig{}).Add(argparse.Arg{Type: argparse.Int}, "--room")
	sub.AddParser("config", argparse.SubparserConfig{}).Add(argparse.Arg{Type: argparse.Int}, "--room")
	sub.AddParser("read", argparse.SubparserConfig{}).Add(argparse.Arg{}, "ord")
	sub.AddParser("dump", argparse.SubparserConfig{}).Add(argparse.Arg{}, "room")
	ns, code, done := p.Parse(args, s)
	if done {
		return code
	}
	return Run(Options{Base: ns.String("base"), PassFile: ns.String("pass_file"), Command: ns.String("cmd"),
		Room: ns.Int("room"), JSON: ns.Bool("json"), Ord: ns.String("ord"), DumpRoom: ns.String("room")}, s)
}
