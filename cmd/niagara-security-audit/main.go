package main

import (
	"github.com/angeles725/niagara-go/internal/cli"
	"github.com/angeles725/niagara-go/internal/secaudit"
)

func main() { cli.Run(secaudit.Main) }
