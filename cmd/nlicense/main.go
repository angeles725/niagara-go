// Command nlicense is the Go port of tools/niagara-license-tool.py.
package main

import (
	"github.com/angeles725/niagara-go/internal/cli"
	"github.com/angeles725/niagara-go/internal/nlicense"
)

func main() { cli.Run(nlicense.Main) }
