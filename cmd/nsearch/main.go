// Command nsearch is the niagara-research search front door (tools/nsearch.py).
package main

import (
	"github.com/angeles725/niagara-go/internal/cli"
	"github.com/angeles725/niagara-go/internal/searchdoor"
)

func main() { cli.Run(searchdoor.Main) }
