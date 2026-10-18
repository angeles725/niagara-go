// Package lookup declares the module-nav lookup commands (Find classes and concepts);
// they register into nav/registry from init.
package lookup

import (
	"github.com/angeles725/niagara-go/internal/argparse"
	"github.com/angeles725/niagara-go/internal/nav/cliargs"
	"github.com/angeles725/niagara-go/internal/nav/registry"
)

func init() {
	registry.Register("lookup", []registry.Command{
		{Name: "class", Help: "Look up a class by exact name", Args: argsClass, Run: runClass},
		{Name: "search", Help: "Search classes by glob pattern", Args: argsSearch, Run: runSearch},
		{Name: "ask", Help: "Same as `rank` (ranked classes for a question); see also: grep", Description: "Answers with `rank`: the classes most relevant to the question, best first. Use `grep` for literal text in the sources.", Args: argsAsk, Run: runAsk},
	})
}

// argsClass declares `class`.
func argsClass(p *argparse.Parser) {
	p.Add(argparse.Arg{Help: "Class name (e.g. BLinkPad)"}, "name")
}

// argsSearch declares `search`.
func argsSearch(p *argparse.Parser) {
	p.Add(argparse.Arg{Help: "Glob pattern (e.g. '*Dialog*', 'BLink*')"}, "pattern")
	p.Add(argparse.Arg{Default: 50, Type: argparse.Int, Help: "Max results to show (default 50)"}, "-n")
}

// argsAsk declares `ask`.
func argsAsk(p *argparse.Parser) {
	p.Add(argparse.Arg{Nargs: argparse.OneOrMore, Help: "Question in natural language (e.g. 'how does alarm routing work')"}, "question")
	p.Add(argparse.Arg{Default: 20, Type: cliargs.PositiveInt, Help: "Max classes to show (default 20)"}, "-n", "--limit")
}
