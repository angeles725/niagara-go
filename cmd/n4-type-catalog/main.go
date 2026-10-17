// Command n4-type-catalog catalogs the Niagara 4 slot declarations of Java source trees.
package main

import (
	"github.com/angeles725/niagara-go/internal/cli"
	"github.com/angeles725/niagara-go/internal/typecatalog"
)

func main() { cli.Run(typecatalog.Main) }
