// Command greenmax-matrix builds the GreenMAX lighting points matrix (Go port of tools/greenmax-matrix.py).
package main

import (
	"github.com/angeles725/niagara-go/internal/cli"
	"github.com/angeles725/niagara-go/internal/greenmaxmatrix"
)

func main() { cli.Run(greenmaxmatrix.Main) }
