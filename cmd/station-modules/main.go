// Command station-modules resolves a station's module dependencies against a Niagara install.
package main

import (
	"github.com/angeles725/niagara-go/internal/cli"
	"github.com/angeles725/niagara-go/internal/stationmodules"
)

func main() { cli.Run(stationmodules.Main) }
