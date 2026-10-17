// Command licensador is the Go port of tools/licensador/licensador.py.
package main

import (
	"github.com/angeles725/niagara-go/internal/cli"
	"github.com/angeles725/niagara-go/internal/licensador"
)

func main() { cli.Run(licensador.Main) }
