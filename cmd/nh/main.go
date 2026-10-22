// Command nh is the niagara-help documentation navigator (niagara_help.py).
// It is the composition root that joins the reader (internal/help) and the
// index builders (internal/helpbuild), which never import each other.
package main

import (
	"github.com/angeles725/niagara-go/internal/cli"
	"github.com/angeles725/niagara-go/internal/help"
	"github.com/angeles725/niagara-go/internal/helpbuild"
)

func main() {
	help.UseBuilders(helpbuild.NH{})
	cli.Run(help.Main)
}
