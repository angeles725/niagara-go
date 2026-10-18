package help

// coverage (cli/cmd_export.py run_coverage), run in process through
// internal/helpbuild: the class and guide coverage report (overview or one
// module), the doc-jar gate (--doc-jars) and the whole-install sweep
// (--sweep, after the doc-jar gate when both are given). helpbuild prints
// the reports byte for byte as Python does; this file only maps them to
// run_coverage's exit statuses. The dependency goes one way: help imports
// helpbuild, never the reverse.

import (
	"fmt"

	"github.com/angeles725/niagara-go/internal/argparse"
	"github.com/angeles725/niagara-go/internal/help/scope"
	"github.com/angeles725/niagara-go/internal/helpbuild"
)

// maintCommands are the maintenance commands that run the index builders
// and gates of internal/helpbuild.
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
	if p.Sweep {
		status := helpbuild.SweepPass
		if p.DocJars {
			st, err := helpbuild.DocJarCoverage(e.Out, e.RepoRoot, e.BaseDir, e.VersionID, e.fulltextDocs)
			if err != nil {
				return err
			}
			fmt.Fprintln(e.Out)
			if st == helpbuild.SweepUnavailable {
				return ExitCode(st)
			}
			status = st
		}
		st, err := helpbuild.RunSweep(e.RepoRoot, e.BaseDir, e.VersionID, e.Out,
			helpbuild.SweepOptions{Fulltext: e.fulltextDocs})
		if err != nil {
			return err
		}
		return ExitCode(max(status, st))
	}
	if p.DocJars {
		st, err := helpbuild.DocJarCoverage(e.Out, e.RepoRoot, e.BaseDir, e.VersionID, e.fulltextDocs)
		if err != nil {
			return err
		}
		return ExitCode(st)
	}
	res, err := helpbuild.Coverage(e.Out, e.BaseDir, p.Module)
	switch {
	case err != nil:
		return err
	case res.Miss:
		return e.reportMiss("coverage", p.Module, "modules")
	case res.GuideGaps > 0:
		return ExitCode(1)
	}
	return nil
}

// fulltextDocs is the helpbuild.FulltextDocs of the gates: the documents of
// the up-to-date full-text index of root. A missing or stale index is built
// first by nh's search opener (open_index's notice on stderr, the build
// lock next to the database), then the fresh database is read.
func (e *Env) fulltextDocs(root string) (map[string]bool, error) {
	x, err := scope.OpenFulltext(e.Err)(root, e.VersionID)
	if err != nil {
		return nil, err
	}
	if err := x.Close(); err != nil {
		return nil, err
	}
	return helpbuild.FreshFulltextDocs(root)
}
