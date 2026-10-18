// Package security declares the module-nav security commands (Security, licensing and permissions);
// they register into nav/registry from init.
package security

import (
	"github.com/angeles725/niagara-go/internal/argparse"
	"github.com/angeles725/niagara-go/internal/nav/registry"
)

func init() {
	registry.Register("security", []registry.Command{
		{Name: "license-feature", Help: "Call sites that check or read a licensed feature (checkFeature/getFeature)", Args: argsLicenseFeature, Run: runLicenseFeature},
		{Name: "license-features", Help: "Licensed feature names with counts; --module: features a module requires", Args: argsLicenseFeatures, Run: runLicenseFeatures},
	})
}

// argsLicenseFeature declares `license-feature`.
func argsLicenseFeature(p *argparse.Parser) {
	p.Add(argparse.Arg{Help: "Feature name (e.g. historyImport)"}, "name")
	p.Add(argparse.Arg{Default: 100, Type: argparse.Int, Help: "Max results to show (default 100)"}, "-n")
	p.Add(argparse.Arg{Help: "Only call sites in this submodule or module (e.g. bacnet-rt, bacnet)", Dest: "lf_module"}, "--module", "-m")
}

// argsLicenseFeatures declares `license-features`.
func argsLicenseFeatures(p *argparse.Parser) {
	p.Add(argparse.Arg{Default: 50, Type: argparse.Int, Help: "Max feature names to show (default 50)"}, "-n")
	p.Add(argparse.Arg{Help: "Show the features this submodule or module requires", Dest: "lf_module"}, "--module", "-m")
	p.Add(argparse.Arg{Action: argparse.StoreTrue, Help: "List every call and getLicenseManager() site no feature name was found for", Dest: "lf_unresolved"}, "--unresolved")
}
