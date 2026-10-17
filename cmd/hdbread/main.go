package main

import (
	"github.com/angeles725/niagara-go/internal/cli"
	"github.com/angeles725/niagara-go/internal/hdbread"
)

func main() { cli.Run(hdbread.Main) }
