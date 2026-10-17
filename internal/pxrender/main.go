package pxrender

import (
	"github.com/angeles725/niagara-go/internal/argparse"
	"github.com/angeles725/niagara-go/internal/cli"
)

// Main is the px-render command line (Python argparse parity). --organized has no default here:
// the Python tool falls back to the organized/ corpus next to its own file.
func Main(args []string, s cli.Stdio) int {
	p := argparse.New(argparse.Config{Prog: "px-render.py",
		Description: "Render a Niagara Px as a self-contained HTML page."})
	p.Add(argparse.Arg{Help: "path to .px file (or 'selftest')"}, "px")
	p.Add(argparse.Arg{Help: "station shared/ root (auto-detected if omitted)"}, "--shared")
	p.Add(argparse.Arg{Help: "organized/ corpus root for module:// resources (repo default)"}, "--organized")
	p.Add(argparse.Arg{Help: "output .html path (default: stdout)"}, "--out")
	p.Add(argparse.Arg{Type: argparse.Float, Default: 0.7, Help: "demo fraction of relays ON (0..1)"}, "--on-ratio")
	p.Add(argparse.Arg{Help: "demo-state seed tag (default: px basename)"}, "--seed")
	p.Add(argparse.Arg{Help: "page <title>"}, "--title")
	p.Add(argparse.Arg{Help: "note line under the header"}, "--subtitle")
	ns, code, done := p.Parse(args, s)
	if done {
		return code
	}
	return Run(Options{Px: ns.String("px"), Shared: ns.String("shared"), Organized: ns.String("organized"),
		Out: ns.String("out"), OnRatio: ns.Float("on_ratio"), Seed: ns.String("seed"), Title: ns.String("title"),
		Subtitle: ns.String("subtitle")}, s)
}
