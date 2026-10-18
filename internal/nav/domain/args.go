// Package domain declares the module-nav domain command (Niagara domain: drivers, the driver
// networks and their anatomy); it registers into nav/registry from init.
package domain

import (
	"github.com/angeles725/niagara-go/internal/argparse"
	"github.com/angeles725/niagara-go/internal/nav/registry"
)

func init() {
	registry.Register("domain", []registry.Command{
		{Name: "drivers", Help: "Driver networks and their anatomy (device, extensions, managers, license)", Description: "One row per driver network (a BDeviceNetwork subclass in an -rt module): family, device, license feature, doc module. With a module: the full anatomy of its networks (devices, point-device-ext, proxy-ext, managers, discovery jobs, license features, doc module).", Epilog: "examples: drivers | drivers modbusTcp-rt | drivers niagaraDriver --json", Args: argsDrivers, Run: runDrivers},
	})
}

// argsDrivers declares `drivers`.
func argsDrivers(p *argparse.Parser) {
	p.Add(argparse.Arg{Nargs: argparse.Optional, Help: "Submodule (modbusTcp-rt) or module directory (modbusTcp)"}, "module")
	p.Add(argparse.Arg{Action: argparse.StoreTrue, Help: "JSON output", Dest: "drivers_json"}, "--json")
}
