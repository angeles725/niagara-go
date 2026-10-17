// Command bog-write generates and validates offline BOG XML for planned station logic.
package main

import (
	"github.com/angeles725/niagara-go/internal/bogwrite"
	"github.com/angeles725/niagara-go/internal/cli"
)

func main() { cli.Run(bogwrite.Main) }
