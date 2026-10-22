package help

import (
	"errors"
	"fmt"
	"io"

	"github.com/angeles725/niagara-go/internal/argparse"
	"github.com/angeles725/niagara-go/internal/cli"
	"github.com/angeles725/niagara-go/internal/index"
	"github.com/angeles725/niagara-go/internal/pycompat"
)

// The writers of `versions`: add (cli/cmd_versions.py over
// version_setup.add_version) and extract (tools/extract_version.py, a Go-only
// subcommand: the script had no nh entry). Both run in process through the
// plugged-in Builders (internal/helpbuild); the full-text index the gates and the summary read is
// built through scope.OpenFulltext, so under its build lock.

// versionsExtractArgs is extract_version.py's argument parser as the
// `versions extract` subcommand.
func versionsExtractArgs(sub *argparse.Subparsers) {
	ext := sub.AddParser("extract", argparse.SubparserConfig{Help: "Extract a registered version's " +
		"doc jars, build its indexes and verify every file (exit 0 verified, 1 the fidelity check " +
		"failed, 2 cannot run)"})
	ext.Add(argparse.Arg{Help: "Registered version id (or a unique prefix)"}, "version")
	ext.Add(argparse.Arg{Action: argparse.StoreTrue, Help: "Only run the sha256 fidelity check and " +
		"write the manifest"}, "--verify-only")
	ext.Add(argparse.Arg{Action: argparse.StoreTrue, Help: "delete existing raw+derived dirs before " +
		"extracting"}, "--force")
	ext.Add(argparse.Arg{Action: argparse.StoreTrue, Help: "Extract without running rebuild-all"}, "--no-index")
	ext.Add(argparse.Arg{Action: argparse.StoreTrue, Help: "only write jar entries whose file is absent " +
		"(never overwrites; allowed on the repo root)"}, "--fill-missing")
}

// addVersion is `versions add <install>`: version_setup.add_version on the
// repository, its lines on stdout, the builders' stderr on stderr; the exit
// status is add_version's return value.
func (e *Env) addVersion(ns *argparse.Namespace) error {
	b, err := theBuilders()
	if err != nil {
		return err
	}
	opt := AddVersionParams{
		Label:    ns.String("label"),
		Default:  ns.Bool("default"),
		Force:    ns.Bool("force"),
		Fulltext: e.fulltextDocs,
	}
	if ns.Get("version_id") != nil { // an explicit --id, even "", is validated, never detected
		id := ns.String("version_id")
		opt.ID = &id
	}
	status, err := b.AddVersion(e.RepoRoot, ns.String("install"), e.Out, e.Err, opt)
	if err != nil {
		return err
	}
	return ExitCode(status)
}

// extractVersion is extract_version.py main(): the version resolved against
// the registry only (no NIAGARA_HELP_VERSION, no default), then
// extract_and_index; its lines on stdout, an error on stderr. Exit 0
// extracted and verified, 1 the fidelity check failed, 2 cannot run.
func (e *Env) extractVersion(ns *argparse.Namespace) error {
	b, err := theBuilders()
	if err != nil {
		return err
	}
	requested := ns.String("version")
	if pycompat.Strip(requested) == "" { // resolve_version would fall back to the default
		fmt.Fprintln(e.Err, "ERROR: empty version id; pass a registered version")
		return ExitCode(cli.ExitCannotSearch)
	}
	vid, err := index.ResolveVersion(e.RepoRoot, requested, func(string) string { return "" })
	var unknown *index.UnknownVersionError
	if errors.As(err, &unknown) {
		fmt.Fprintf(e.Err, "ERROR: %s\n", unknown.Msg)
		return ExitCode(cli.ExitCannotSearch)
	} else if err != nil {
		return err
	}
	opt := ExtractParams{Force: ns.Bool("force"), VerifyOnly: ns.Bool("verify_only"),
		FillMissing: ns.Bool("fill_missing")}
	var rebuildLog io.Writer
	if !ns.Bool("no_index") {
		rebuildLog = e.Err
	}
	lines, status, errMsg, err := b.ExtractVersion(e.RepoRoot, vid, rebuildLog, opt)
	for _, line := range lines {
		fmt.Fprintln(e.Out, line)
	}
	if err != nil {
		return err
	}
	if errMsg != "" { // an error is never a success, whatever status came with it
		fmt.Fprintf(e.Err, "ERROR: %s\n", errMsg)
		return ExitCode(max(status, cli.ExitCannotSearch))
	}
	return ExitCode(status)
}
