package obixnav

import (
	"github.com/angeles725/niagara-go/internal/argparse"
	"github.com/angeles725/niagara-go/internal/cli"
)

// DefaultBase is the SSH tunnel to the PANCCADIA JACE. The pass file has no fixed default:
// --pass-file falls back to the PassFileEnv environment variable.
const (
	DefaultBase = "https://127.0.0.1:18443/obix"
	PassFileEnv = "OBIX_PASS_FILE"
)

// Main is the obix-nav command line (Python argparse parity).
func Main(args []string, s cli.Stdio) int {
	o, code, done := parse(args, s)
	if done {
		return code
	}
	return Run(o, s)
}

// parse reads the command line into Options; done is true (with the exit code) after --help or a
// usage error.
func parse(args []string, s cli.Stdio) (o Options, code int, done bool) {
	p := argparse.New(argparse.Config{Prog: "obix-nav"})
	p.Add(argparse.Arg{Default: DefaultBase}, "--base")
	p.Add(argparse.Arg{Default: s.Env(PassFileEnv)}, "--pass-file")
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
		return o, code, true
	}
	o = Options{Base: ns.String("base"), PassFile: ns.String("pass_file"), Command: ns.String("cmd"),
		JSON: ns.Bool("json"), Ord: ns.String("ord")}
	// "room" is --room (an int, None when absent) for map/wiring/config and the dump positional (a
	// string): read it once, by the type its subcommand stores.
	switch v := ns.Get("room").(type) {
	case int:
		o.Room = v
	case string:
		o.DumpRoom = v
	}
	return o, 0, false
}
