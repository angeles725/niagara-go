package lookup

import (
	"fmt"
	"strings"

	"github.com/angeles725/niagara-go/internal/argparse"
	"github.com/angeles725/niagara-go/internal/nav/classrank"
	"github.com/angeles725/niagara-go/internal/nav/registry"
)

// runAsk is commands/lookup.py run_ask: `rank` under another name, with a NOTE.
func runAsk(env *registry.Env, ns *argparse.Namespace) int {
	fmt.Fprint(env.Err, "NOTE: ask answers with `rank` (ranked classes); use grep for literal text.\n")
	classrank.Cmd(env, strings.Join(ns.Strings("question"), " "), ns.Int("limit"))
	return env.ExitCode()
}
