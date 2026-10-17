// Package ui declares the module-nav ui commands (UI and Swing);
// they register into nav/registry from init.
package ui

import (
	"github.com/angeles725/niagara-go/internal/argparse"
	"github.com/angeles725/niagara-go/internal/nav/registry"
)

func init() {
	registry.Register("ui", []registry.Command{
		{Name: "ui", Help: "UI/Swing pattern search (dialogs, sizes, colors, fonts)", Args: argsUI},
	})
}

// argsUI declares `ui`.
func argsUI(p *argparse.Parser) {
	s := p.AddSubparsers(argparse.SubparsersConfig{Dest: "ui_command", Help: "UI sub-commands"})
	argsUIDialogs(s.AddParser("dialogs", argparse.SubparserConfig{Help: "List all dialog classes"}))
	argsUISizes(s.AddParser("sizes", argparse.SubparserConfig{Help: "List all hardcoded dimensions"}))
	argsUIColors(s.AddParser("colors", argparse.SubparserConfig{Help: "List all hardcoded colors"}))
	argsUIFonts(s.AddParser("fonts", argparse.SubparserConfig{Help: "List all hardcoded fonts"}))
	argsUIClass(s.AddParser("class", argparse.SubparserConfig{Help: "Full UI details for a specific class"}))
	s.AddParser("customizable", argparse.SubparserConfig{Help: "Summary of all patcheable UI elements"})
}

// argsUIDialogs declares `ui dialogs`.
func argsUIDialogs(p *argparse.Parser) {
	p.Add(argparse.Arg{Default: 50, Type: argparse.Int, Help: "Max results to show (default 50)"}, "-n")
}

// argsUISizes declares `ui sizes`.
func argsUISizes(p *argparse.Parser) {
	p.Add(argparse.Arg{Default: 50, Type: argparse.Int, Help: "Max results to show (default 50)"}, "-n")
}

// argsUIColors declares `ui colors`.
func argsUIColors(p *argparse.Parser) {
	p.Add(argparse.Arg{Default: 50, Type: argparse.Int, Help: "Max results to show (default 50)"}, "-n")
}

// argsUIFonts declares `ui fonts`.
func argsUIFonts(p *argparse.Parser) {
	p.Add(argparse.Arg{Default: 50, Type: argparse.Int, Help: "Max results to show (default 50)"}, "-n")
}

// argsUIClass declares `ui class`.
func argsUIClass(p *argparse.Parser) {
	p.Add(argparse.Arg{Help: "Class name (e.g. BLinkPad)"}, "name")
}
