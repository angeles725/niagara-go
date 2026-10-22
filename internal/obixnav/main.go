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
	p.Add(argparse.Arg{Default: DefaultBase, Help: "oBIX root (default: the PANCCADIA JACE tunnel " + DefaultBase + ")"}, "--base")
	p.Add(argparse.Arg{Default: s.Env(PassFileEnv), Help: "file holding the API user's password (default: $" + PassFileEnv + ")"}, "--pass-file")
	sub := p.AddSubparsers(argparse.SubparsersConfig{Dest: "cmd", Required: true})
	room := argparse.Arg{Type: argparse.Int, Help: "only this room (CuartoN); default every room"}
	m := sub.AddParser("map", command("audit the per-evap facade -> rt EvaporatorUnit mapping (control and defrost slots)"))
	m.Add(room, "--room")
	m.Add(argparse.Arg{Action: argparse.StoreTrue, Help: "accepted for compatibility; no effect"}, "--json")
	sub.AddParser("cond", command("compare the Condensadoras facade with Programacion/CompressorControl"))
	sub.AddParser("wiring", command("list rt control slots of each EvaporatorUnit that have no link at all")).Add(room, "--room")
	sub.AddParser("config", command("compare the per-unit and defrost configuration slots of each room")).Add(room, "--room")
	sub.AddParser("read", command("batch-read one ord and print its (tag, value)")).Add(argparse.Arg{Help: "station ord, e.g. /Services/DashboardService/Cuarto1/evapTemp1"}, "ord")
	sub.AddParser("dump", command("print every facade slot of one room and its rt units")).Add(argparse.Arg{Help: "room number N of CuartoN"}, "room")
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

// command is the configuration of one subcommand: its line in the parent's command list and the
// description of its own --help.
func command(help string) argparse.SubparserConfig {
	return argparse.SubparserConfig{Help: help, Config: argparse.Config{Description: help}}
}
