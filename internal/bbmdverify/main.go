package bbmdverify

import (
	"github.com/angeles725/niagara-go/internal/argparse"
	"github.com/angeles725/niagara-go/internal/cli"
)

// Prog keeps the Python script name so usage and errors match the original.
const Prog = "bacnet-bbmd-verify.py"

func newParser() *argparse.Parser {
	p := argparse.New(argparse.Config{Prog: Prog,
		Description: "Verify BACnet/IP BBMD + Who-Is/I-Am cross-subnet reachability (read-only)."})
	p.Add(argparse.Arg{Required: true, Help: "a known field device on the OTHER subnet (e.g. 192.168.0.50)"}, "--device-ip")
	p.Add(argparse.Arg{Type: argparse.Int, Default: 24, Help: "prefix length of the device subnet (default 24)"}, "--device-prefix")
	p.Add(argparse.Arg{Default: "255.255.255.255", Help: "broadcast address of THIS host's interface (default limited broadcast)"}, "--local-bcast")
	p.Add(argparse.Arg{Help: "IP to probe as a BBMD (Read-BDT/Read-FDT)"}, "--bbmd")
	p.Add(argparse.Arg{Type: argparse.Int, Default: 47808, Help: "BACnet/IP UDP port (default 47808)"}, "--port")
	p.Add(argparse.Arg{Type: argparse.Int, Default: 47808, Help: "local UDP bind port (use 0 if Niagara holds 47808 on this host)"}, "--src-port")
	p.Add(argparse.Arg{Type: argparse.Float, Default: 3.0, Help: "per-probe listen window (s)"}, "--timeout")
	p.Add(argparse.Arg{Action: argparse.StoreTrue, Help: "machine-readable output"}, "--json")
	return p
}

// Main is the bacnet-bbmd-verify entry point.
func Main(args []string, s cli.Stdio) int {
	ns, code, done := newParser().Parse(args, s)
	if done {
		return code
	}
	return Run(Options{
		DeviceIP:     ns.String("device_ip"),
		DevicePrefix: ns.Int("device_prefix"),
		LocalBcast:   ns.String("local_bcast"),
		BBMD:         ns.String("bbmd"),
		Port:         ns.Int("port"),
		SrcPort:      ns.Int("src_port"),
		Timeout:      ns.Float("timeout"),
		JSON:         ns.Bool("json"),
	}, s)
}
