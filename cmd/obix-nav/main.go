// Command obix-nav is a read-only facade<->rt auditor for the PANCCADIA JACE over oBIX (Go port of tools/obix-nav.py).
package main

import (
	"github.com/angeles725/niagara-go/internal/cli"
	"github.com/angeles725/niagara-go/internal/obixnav"
)

func main() { cli.Run(obixnav.Main) }
