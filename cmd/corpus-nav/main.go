// Command corpus-nav navigates the niagara-research knowledge corpus (tools/corpus-nav.py).
package main

import (
	"github.com/angeles725/niagara-go/internal/cli"
	"github.com/angeles725/niagara-go/internal/corpusnav"
)

func main() { cli.Run(corpusnav.Main) }
