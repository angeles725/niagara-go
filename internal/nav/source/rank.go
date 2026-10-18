package source

import (
	"strings"

	"github.com/angeles725/niagara-go/internal/argparse"
	"github.com/angeles725/niagara-go/internal/nav/rankcmd"
	"github.com/angeles725/niagara-go/internal/nav/registry"
)

// runRank is commands/source.py run_rank.
func runRank(env *registry.Env, ns *argparse.Namespace) int {
	rankcmd.Cmd(env, strings.Join(ns.Strings("question"), " "), ns.Int("limit"))
	return env.ExitCode()
}
