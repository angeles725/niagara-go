package help

import (
	"errors"
	"fmt"

	"github.com/angeles725/niagara-go/internal/argparse"
	"github.com/angeles725/niagara-go/internal/cli"
	"github.com/angeles725/niagara-go/internal/help/scope"
	"github.com/angeles725/niagara-go/internal/helpbuild"
	"github.com/angeles725/niagara-go/internal/index"
	"github.com/angeles725/niagara-go/internal/pycompat"
)

// The writers of `versions`: add (cli/cmd_versions.py over
// version_setup.add_version) and extract (tools/extract_version.py, a Go-only
// subcommand: the script had no nh entry). Both run in process through
// internal/helpbuild; the full-text index the gates and the summary read is
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

// fulltextDocs is the FulltextDocs of nh: open_index(root).docs(), the index
// built first when it is missing or stale (scope.OpenFulltext: the building
// notes on stderr, the build lock). The handle is closed, releasing the lock,
// before the docs are read: safe because every fulltext build writes a temp
// file and renames it into place (helpbuild/fulltext build.go), so a reader
// never sees a half-written index.
func (e *Env) fulltextDocs(root string) (map[string]bool, error) {
	x, err := scope.OpenFulltext(e.Err)(root, "")
	if err != nil {
		return nil, err
	}
	if err := x.Close(); err != nil {
		return nil, err
	}
	path, err := helpbuild.FreshFulltextPath(root)
	if err != nil {
		return nil, err
	}
	return helpbuild.FulltextDocsAt(path)
}

// addVersion is `versions add <install>`: version_setup.add_version on the
// repository, its lines on stdout, the builders' stderr on stderr; the exit
// status is add_version's return value.
func (e *Env) addVersion(ns *argparse.Namespace) error {
	opt := helpbuild.AddOptions{
		Label:    ns.String("label"),
		Default:  ns.Bool("default"),
		Force:    ns.Bool("force"),
		Rebuild:  helpbuild.RebuildToLog(e.Err),
		Fulltext: e.fulltextDocs,
	}
	if ns.Get("version_id") != nil { // an explicit --id, even "", is validated, never detected
		id := ns.String("version_id")
		opt.ID = &id
	}
	status, err := helpbuild.AddVersion(e.RepoRoot, ns.String("install"), e.Out, opt)
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
	opt := helpbuild.SetupOptions{Force: ns.Bool("force"), VerifyOnly: ns.Bool("verify_only"),
		FillMissing: ns.Bool("fill_missing")}
	if !ns.Bool("no_index") {
		opt.Rebuild = helpbuild.RebuildToLog(e.Err)
	}
	res, err := helpbuild.ExtractAndIndex(e.RepoRoot, vid, opt)
	for _, line := range res.Log {
		fmt.Fprintln(e.Out, line)
	}
	if err != nil {
		return err
	}
	if res.Error != "" { // an error is never a success, whatever status came with it
		fmt.Fprintf(e.Err, "ERROR: %s\n", res.Error)
		return ExitCode(max(res.Status, cli.ExitCannotSearch))
	}
	return ExitCode(res.Status)
}
