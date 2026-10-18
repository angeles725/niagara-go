// Package audits declares the module-nav audits commands (Module audits);
// they register into nav/registry from init.
package audits

import (
	"github.com/angeles725/niagara-go/internal/argparse"
	"github.com/angeles725/niagara-go/internal/nav/registry"
)

func init() {
	registry.Register("audits", []registry.Command{
		{Name: "palette-lexicon-agents", Help: "Palette entries, lexicon keys (+ dup-key report), and agent registrations for a module", Args: argsPaletteLexiconAgents, Run: runPaletteLexiconAgents},
	})
}

// argsPaletteLexiconAgents declares `palette-lexicon-agents`. The parser rides
// in the namespace so the command can report "give a module name or --all"
// as the script's p.error does.
func argsPaletteLexiconAgents(p *argparse.Parser) {
	p.Add(argparse.Arg{Nargs: argparse.Optional, Help: "Module name in organized/ (e.g. alarm)"}, "module")
	p.Add(argparse.Arg{Action: argparse.StoreTrue, Help: "Census of every module under the base directory (summary table)", Dest: "pla_all"}, "--all")
	p.Add(argparse.Arg{Action: argparse.StoreTrue, Help: "Structured JSON output", Dest: "pla_json"}, "--json")
	p.SetDefaults("pla_parser", p)
}
