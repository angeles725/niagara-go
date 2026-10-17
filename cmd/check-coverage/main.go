// Command check-coverage is the prior-coverage gate (tools/check-coverage.py).
package main

import (
	"github.com/angeles725/niagara-go/internal/checkcoverage"
	"github.com/angeles725/niagara-go/internal/cli"
)

func main() { cli.Run(checkcoverage.Main) }
