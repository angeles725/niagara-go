// Command module-find is a read-only finder over a Niagara N4 module's Java sources
// (tools/module-find.py).
package main

import (
	"github.com/angeles725/niagara-go/internal/cli"
	"github.com/angeles725/niagara-go/internal/modulefind"
)

func main() { cli.Run(modulefind.Main) }
