package helpbuild

import (
	"io"

	"github.com/angeles725/niagara-go/internal/help/buildapi"
)

// NH is the builders nh's maintenance commands run (buildapi.Builders): each
// method is the helpbuild entry point those commands called in process
// before the reader stopped importing helpbuild, so the output and the exit
// statuses are unchanged. cmd/nh plugs it in with help.UseBuilders; helpbuild
// imports only the contract (buildapi), never the reader.
type NH struct{}

var _ buildapi.Builders = NH{}

// Coverage is Coverage with its result spread out.
func (NH) Coverage(w io.Writer, baseDir, module string) (bool, int, error) {
	res, err := Coverage(w, baseDir, module)
	return res.Miss, res.GuideGaps, err
}

// DocJarCoverage is DocJarCoverage.
func (NH) DocJarCoverage(w io.Writer, repoRoot, baseDir, versionID string,
	fulltext func(root string) (map[string]bool, error)) (int, error) {
	return DocJarCoverage(w, repoRoot, baseDir, versionID, fulltext)
}

// Sweep is RunSweep with the given full-text documents.
func (NH) Sweep(w io.Writer, repoRoot, baseDir, versionID string,
	fulltext func(root string) (map[string]bool, error)) (int, error) {
	return RunSweep(repoRoot, baseDir, versionID, w, SweepOptions{Fulltext: fulltext})
}

// FreshFulltextDocs is FreshFulltextDocs.
func (NH) FreshFulltextDocs(baseDir string) (map[string]bool, error) {
	return FreshFulltextDocs(baseDir)
}

// RebuildAll is RebuildAll; the counts are printed, not returned.
func (NH) RebuildAll(baseDir string, p buildapi.RebuildParams) error {
	_, err := RebuildAll(baseDir, RebuildOptions{Only: p.Only, DryRun: p.DryRun,
		StampOnly: p.StampOnly, Stdout: p.Stdout, Stderr: p.Stderr})
	return err
}

// AddVersion is AddVersion rebuilding through RebuildToLog(log).
func (NH) AddVersion(repoRoot, install string, w, log io.Writer, p buildapi.AddVersionParams) (int, error) {
	return AddVersion(repoRoot, install, w, AddOptions{Label: p.Label, Default: p.Default,
		Force: p.Force, ID: p.ID, Rebuild: RebuildToLog(log), Fulltext: p.Fulltext})
}

// ExtractVersion is ExtractAndIndex, rebuilding through
// RebuildToLog(rebuildLog) unless rebuildLog is nil.
func (NH) ExtractVersion(repoRoot, versionID string, rebuildLog io.Writer,
	p buildapi.ExtractParams) ([]string, int, string, error) {
	opt := SetupOptions{Force: p.Force, VerifyOnly: p.VerifyOnly, FillMissing: p.FillMissing}
	if rebuildLog != nil {
		opt.Rebuild = RebuildToLog(rebuildLog)
	}
	res, err := ExtractAndIndex(repoRoot, versionID, opt)
	if res == nil {
		return nil, 0, "", err
	}
	return res.Log, res.Status, res.Error, err
}
