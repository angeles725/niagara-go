// Command dashboard-preview is the Go port of tools/dashboard-preview.py.
package main

import (
	"github.com/angeles725/niagara-go/internal/cli"
	"github.com/angeles725/niagara-go/internal/dashpreview"
)

func main() { cli.Run(dashpreview.Main) }
