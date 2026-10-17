package help

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/angeles725/niagara-go/internal/argparse"
	"github.com/angeles725/niagara-go/internal/cli"
	"github.com/angeles725/niagara-go/internal/index"
)

// Version management (cli/cmd_versions.py): versions add, versions list.
// `versions` runs before any version is resolved (a new install has no data
// root yet), so Main does not map --niagara-version for it.
var versionCommands = []command{
	{name: "versions", help: "Manage Niagara versions: 'versions add <install>' onboards a new " +
		"install in one command (exit 0 ready, 1 a gate failed, 2 cannot " +
		"add); 'versions list'",
		args: versionsArgs, run: runVersions},
}

func versionsArgs(p *argparse.Parser) {
	sub := p.AddSubparsers(argparse.SubparsersConfig{Dest: "action", Metavar: "{add,list}"})
	sub.SetRequired(true)
	add := sub.AddParser("add", argparse.SubparserConfig{Help: "Register a Niagara install, " +
		"extract and index all of its documentation, run both gates and print a summary"})
	add.Add(argparse.Arg{Help: "Path of the Niagara install (the folder with modules/)"}, "install")
	add.Add(argparse.Arg{Help: "Human name (default: the install folder name)"}, "--label")
	add.Add(argparse.Arg{Action: argparse.StoreTrue, Help: "Make it the default version"}, "--default")
	add.Add(argparse.Arg{Dest: "version_id", Help: "Version id (a.b.c.d) when it cannot be detected " +
		"from modules/baja.jar or the folder name"}, "--id")
	add.Add(argparse.Arg{Action: argparse.StoreTrue, Help: "Extract and index an already " +
		"registered version again (same install)"}, "--force")
	sub.AddParser("list", argparse.SubparserConfig{Help: "Registered versions, their data roots " +
		"and whether they are extracted"})
}

func runVersions(e *Env, ns *argparse.Namespace) error {
	if ns.String("action") == "add" {
		return e.addVersion(ns.String("install"))
	}
	return e.listVersions()
}

// addVersion is version_setup.add_version up to its first refusal: the path
// must be a Niagara install (hold modules/). Registering, extracting and
// indexing are writers ported in a later phase.
func (e *Env) addVersion(install string) error {
	abs, err := filepath.Abs(install)
	if err != nil {
		return err
	}
	if st, err := os.Stat(filepath.Join(abs, "modules")); err != nil || !st.IsDir() {
		fmt.Fprintf(e.Out, "ERROR: %s is not a Niagara install (no modules/ folder)\n", abs)
		return ExitCode(cli.ExitCannotSearch)
	}
	fmt.Fprintln(e.Err, "nh: 'versions add' is not ported yet")
	return ExitCode(cli.ExitCannotSearch)
}

// listVersions prints every registered version, its install (and whether it
// is mounted) and its data root (and whether it is extracted); exit 1 when
// none is registered.
func (e *Env) listVersions() error {
	reg, err := index.LoadRegistry(e.RepoRoot)
	if err != nil {
		return err
	}
	if reg == nil || reg.Versions.Len() == 0 {
		fmt.Fprintf(e.Out, "No version registered (%s); add one with 'versions add <install>'.\n",
			index.RegistryRel)
		return ExitCode(cli.ExitNoMatch)
	}
	for _, id := range reg.IDs() {
		entry, _ := reg.Entry(id)
		root, err := index.DataRoot(e.RepoRoot, id)
		if err != nil {
			return err
		}
		def := ""
		if id == reg.Default {
			def = " (default)"
		}
		fmt.Fprintf(e.Out, "%s%s  %s\n", id, def, entry.Label)
		mounted := "  (not mounted)"
		if st, err := os.Stat(entry.InstallDir); err == nil && st.IsDir() {
			mounted = ""
		}
		fmt.Fprintf(e.Out, "    install:   %s%s\n", entry.InstallDir, mounted)
		state := "not extracted"
		if index.IsExtracted(root) {
			state = "extracted"
		}
		fmt.Fprintf(e.Out, "    data root: %s  (%s)\n", entry.DataRoot, state)
	}
	return nil
}
