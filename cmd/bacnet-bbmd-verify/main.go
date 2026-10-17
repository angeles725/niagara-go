// Command bacnet-bbmd-verify is the Go port of tools/bacnet-bbmd-verify.py.
package main

import (
	"github.com/angeles725/niagara-go/internal/bbmdverify"
	"github.com/angeles725/niagara-go/internal/cli"
)

func main() { cli.Run(bbmdverify.Main) }
