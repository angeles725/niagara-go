// Command px-render renders a Niagara Px as a self-contained HTML page (Go port of tools/px-render.py).
package main

import (
	"github.com/angeles725/niagara-go/internal/cli"
	"github.com/angeles725/niagara-go/internal/pxrender"
)

func main() { cli.Run(pxrender.Main) }
