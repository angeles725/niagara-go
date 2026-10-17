// Command gen-catalog writes, checks or prints CATALOG.md (tools/gen-catalog.py).
package main

import (
	"github.com/angeles725/niagara-go/internal/cli"
	"github.com/angeles725/niagara-go/internal/gencatalog"
)

func main() { cli.Run(gencatalog.Main) }
