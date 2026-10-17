// Command niagara-security-audit audits the security posture of a Niagara N4 station/platform (Go port of tools/niagara-security-audit.py).
package main

import (
	"github.com/angeles725/niagara-go/internal/cli"
	"github.com/angeles725/niagara-go/internal/secaudit"
)

func main() { cli.Run(secaudit.Main) }
