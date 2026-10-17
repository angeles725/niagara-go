// Command greenmax-tablero-gen generates a redesigned GreenMAX tablero Px (Go port of tools/greenmax-tablero-gen.py).
package main

import (
	"github.com/angeles725/niagara-go/internal/cli"
	"github.com/angeles725/niagara-go/internal/greenmaxtablero"
)

func main() { cli.Run(greenmaxtablero.Main) }
