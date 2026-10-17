// Command nh is the niagara-help documentation navigator (niagara_help.py).
package main

import (
	"github.com/angeles725/niagara-go/internal/cli"
	"github.com/angeles725/niagara-go/internal/help"
)

func main() { cli.Run(help.Main) }
