// Command mnav is module-nav, the Niagara N4 decompiled-module navigator.
// It is the composition root that joins the reader (internal/nav) and the
// index builder (internal/navbuild/reindex), which the reader never imports.
package main

import (
	"github.com/angeles725/niagara-go/internal/cli"
	"github.com/angeles725/niagara-go/internal/nav"
	"github.com/angeles725/niagara-go/internal/navbuild/reindex"
)

func main() {
	nav.UseReindex(reindex.Main)
	cli.Run(nav.Main)
}
