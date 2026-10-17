// Command bog-cycles reports potential feedback loops in a station bog's link graph.
package main

import (
	"github.com/angeles725/niagara-go/internal/bogcycles"
	"github.com/angeles725/niagara-go/internal/cli"
)

func main() { cli.Run(bogcycles.Main) }
