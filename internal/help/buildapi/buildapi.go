// Package buildapi is the contract between nh's reader (internal/help) and
// its index builders (internal/helpbuild): the operations the maintenance
// commands run and their parameters, in standard-library types only, so
// neither side imports the other.
package buildapi

import "io"

// Builders is the index-building side nh's maintenance commands run
// (coverage, rebuild-all, versions add and versions extract): helpbuild.NH
// implements it, and the composition root (cmd/nh) plugs it in with
// help.UseBuilders, so help and helpbuild sit side by side in the package
// graph, joined only in main.
//
// fulltext is the documents of the up-to-date full-text index of a data
// root (help's Env.fulltextDocs); the int results are the builders' exit
// statuses.
type Builders interface {
	// Coverage prints the class and guide coverage report of baseDir (the
	// overview when module is ""); miss is a module that matches no class,
	// guideGaps the guide folders with unindexed pages.
	Coverage(w io.Writer, baseDir, module string) (miss bool, guideGaps int, err error)
	// DocJarCoverage is the --doc-jars gate: 0 pass, 1 gaps, SweepUnavailable.
	DocJarCoverage(w io.Writer, repoRoot, baseDir, versionID string,
		fulltext func(root string) (map[string]bool, error)) (int, error)
	// Sweep is the --sweep gate over the whole install.
	Sweep(w io.Writer, repoRoot, baseDir, versionID string,
		fulltext func(root string) (map[string]bool, error)) (int, error)
	// FreshFulltextDocs reads the documents of baseDir's full-text index,
	// an error when it is missing or stale.
	FreshFulltextDocs(baseDir string) (map[string]bool, error)
	// RebuildAll is rebuild-all on baseDir: the plan only with dryRun, the
	// manifest stamp only with stampOnly.
	RebuildAll(baseDir string, opt RebuildParams) error
	// AddVersion is `versions add`: its lines on w, the rebuild's stderr on
	// log; the int is add_version's return value.
	AddVersion(repoRoot, install string, w, log io.Writer, opt AddVersionParams) (int, error)
	// ExtractVersion is extract_and_index of a resolved version: the rebuild
	// runs (its stderr on rebuildLog) unless rebuildLog is nil. lines is the
	// setup log, errMsg a refusal ("" when none) and status its exit status.
	ExtractVersion(repoRoot, versionID string, rebuildLog io.Writer, opt ExtractParams) (lines []string, status int, errMsg string, err error)
}

// SweepPass and SweepUnavailable are the coverage gates' statuses help
// reads: nothing unclassified, and the install is not available (the gate
// was not evaluated). They equal helpbuild's SweepPass and SweepUnavailable.
const (
	SweepPass        = 0
	SweepUnavailable = 2
)

// RebuildParams are rebuild-all's arguments.
type RebuildParams struct {
	Only              string
	DryRun, StampOnly bool
	Stdout, Stderr    io.Writer
}

// AddVersionParams are `versions add`'s options; ID nil is "detect the
// version", an explicit "" is validated, never detected.
type AddVersionParams struct {
	Label          string
	Default, Force bool
	ID             *string
	Fulltext       func(root string) (map[string]bool, error)
}

// ExtractParams are `versions extract`'s flags.
type ExtractParams struct {
	Force, VerifyOnly, FillMissing bool
}
