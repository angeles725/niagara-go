// Command bog-nav navigates a Niagara N4 station config.bog (or a bare file.xml), read-only.
package main

import (
	"github.com/angeles725/niagara-go/internal/bognav"
	"github.com/angeles725/niagara-go/internal/cli"
)

func main() { cli.Run(bognav.Main) }
