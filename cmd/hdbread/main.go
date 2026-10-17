// Command hdbread is a read-only reader for Niagara N4 .hdb history files (Go port of tools/hdbread.py).
package main

import (
	"github.com/angeles725/niagara-go/internal/cli"
	"github.com/angeles725/niagara-go/internal/hdbread"
)

func main() { cli.Run(hdbread.Main) }
