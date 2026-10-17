// Command find3 is the alias of the search front door (tools/find3.py).
package main

import (
	"github.com/angeles725/niagara-go/internal/cli"
	"github.com/angeles725/niagara-go/internal/searchdoor"
)

func main() { cli.Run(searchdoor.Find3Main) }
