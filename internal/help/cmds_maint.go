package help

// coverage (cli/cmd_export.py run_coverage) and rebuild-all
// (cli/cmd_admin.py run_rebuild_all), run in process through
// the plugged-in Builders (internal/helpbuild): the class and guide coverage
// report (overview or one module), the doc-jar gate (--doc-jars), the
// whole-install sweep (--sweep, after the doc-jar gate when both are given)
// and the index rebuild with its manifest stamp. helpbuild prints byte for
// byte what Python prints; this file maps the results to Python's exit
// statuses and takes the build lock around a rebuild. Neither package
// imports the other: cmd/nh joins them (builders.go).

import (
	"fmt"

	"github.com/angeles725/niagara-go/internal/argparse"
	"github.com/angeles725/niagara-go/internal/help/scope"
)

// maintCommands are the maintenance commands that run the index builders
// and gates (Builders).
var maintCommands = []command{
	{name: "coverage", help: "Documentation coverage report (overview exits 1 on guide " +
		"folders with unindexed HTML pages)",
		args: func(p *argparse.Parser) {
			optionalPositional(p, "module", nil, "Module name (omit for overview)")
			flag(p, "Gate: every entry of every doc jar of this version must be "+
				"indexed or excluded with a reason (exit 1 on any gap)", "--doc-jars")
			flag(p, "Gate: every documentation-like entry of every jar of the "+
				"whole install (modules/, javadoc/, sw/, root) and every file "+
				"of its docs/ must be indexed or excluded by a rule of "+
				"install_docs.json (exit 1 on any unclassified entry)", "--sweep")
		},
		run: func(e *Env, ns *argparse.Namespace) error {
			return e.Coverage(CoverageParams{Module: ns.String("module"),
				DocJars: ns.Bool("doc_jars"), Sweep: ns.Bool("sweep")})
		}},
	{name: "rebuild-all", help: "Rebuild all indexes in dependency order, then stamp the manifest",
		args: func(p *argparse.Parser) {
			p.Add(argparse.Arg{Help: "Only run builders whose script starts with this"}, "--only")
			flag(p, "Print the build plan without executing", "--dry-run")
			flag(p, "Record current corpus as the baseline without rebuilding", "--stamp-only")
		},
		run: func(e *Env, ns *argparse.Namespace) error {
			return e.RebuildAll(RebuildParams{Only: ns.String("only"),
				DryRun: ns.Bool("dry_run"), StampOnly: ns.Bool("stamp_only")})
		}},
}

// CoverageParams are the coverage arguments: the module ("" is the
// overview, as Python's None and its empty string are) and the two gates.
type CoverageParams struct {
	Module         string
	DocJars, Sweep bool
}

// Coverage is run_coverage. --sweep wins over the module: the doc-jar gate
// first when --doc-jars is given (then a blank line, and exit 2 at once
// when its install is not available), then the sweep, exiting with the
// worse status. --doc-jars alone exits with the gate's status. The report
// exits 1 when the overview finds a guide folder with unindexed pages, and
// a module that matches no class is a miss (exit 1). A missing report index
// is "ERROR: ..." and exit 2, as Python's MissingIndexError.
func (e *Env) Coverage(p CoverageParams) error {
	b, err := theBuilders()
	if err != nil {
		return err
	}
	if p.Sweep {
		status := SweepPass
		if p.DocJars {
			st, err := b.DocJarCoverage(e.Out, e.RepoRoot, e.BaseDir, e.VersionID, e.fulltextDocs)
			if err != nil {
				return err
			}
			fmt.Fprintln(e.Out)
			if st == SweepUnavailable {
				return ExitCode(st)
			}
			status = st
		}
		st, err := b.Sweep(e.Out, e.RepoRoot, e.BaseDir, e.VersionID, e.fulltextDocs)
		if err != nil {
			return err
		}
		return ExitCode(max(status, st))
	}
	if p.DocJars {
		st, err := b.DocJarCoverage(e.Out, e.RepoRoot, e.BaseDir, e.VersionID, e.fulltextDocs)
		if err != nil {
			return err
		}
		return ExitCode(st)
	}
	miss, guideGaps, err := b.Coverage(e.Out, e.BaseDir, p.Module)
	switch {
	case err != nil:
		return err
	case miss:
		return e.reportMiss("coverage", p.Module, "modules")
	case guideGaps > 0:
		return ExitCode(1)
	}
	return nil
}

// fulltextDocs is the full-text documents of the gates: the documents of
// the up-to-date full-text index of root. A missing or stale index is built
// first by nh's search opener (open_index's notice on stderr, the build
// lock next to the database), then the fresh database is read. The handle
// is closed, releasing the lock, before the docs are read: safe because every
// full-text build writes a temp file and renames it into place
// (helpbuild/fulltext build.go), so a reader never sees a half-written index.
// Shared by coverage --sweep and versions add.
func (e *Env) fulltextDocs(root string) (map[string]bool, error) {
	x, err := scope.OpenFulltext(e.Err)(root, e.VersionID, "")
	if err != nil {
		return nil, err
	}
	if err := x.Close(); err != nil {
		return nil, err
	}
	b, err := theBuilders()
	if err != nil {
		return nil, err
	}
	return b.FreshFulltextDocs(root)
}

// RebuildAll is run_rebuild_all: the builders' RebuildAll on the data root,
// printing on nh's streams. Anything that writes (a build, --stamp-only)
// runs under the build lock of the full-text index, so a search waits
// instead of indexing a half-rewritten corpus; a dry run writes nothing and
// takes no lock. --stamp-only wins over --dry-run, as in Python. Failed
// builders still exit 0 (they are counted and the manifest is kept);
// an error (a stream or the manifest that cannot be written, the lock held
// by a live process) is "ERROR: ..." and exit 2.
func (e *Env) RebuildAll(opt RebuildParams) error {
	b, err := theBuilders()
	if err != nil {
		return err
	}
	opt.Stdout, opt.Stderr = e.Out, e.Err
	rebuild := func() error { return b.RebuildAll(e.BaseDir, opt) }
	if opt.DryRun && !opt.StampOnly {
		return rebuild()
	}
	return scope.WithBuildLock(e.BaseDir, rebuild)
}
