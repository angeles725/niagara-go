// Command qnx6read is a minimal read-only QNX6 (Power-Safe) reader for the JACE-8000 SD image (Go port of tools/qnx6read.py).
package main

import (
	"github.com/angeles725/niagara-go/internal/cli"
	"github.com/angeles725/niagara-go/internal/qnx6read"
)

func main() { cli.Run(qnx6read.Main) }
