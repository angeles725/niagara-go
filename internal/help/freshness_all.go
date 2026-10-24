package help

// freshness --all: one line per registered Niagara version (its indexes against
// their baseline, its install's doc jars against versions/<id>.manifest.json) and
// a last line counting the verdicts, which the exit status follows.

import (
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/angeles725/niagara-go/internal/cli"
	"github.com/angeles725/niagara-go/internal/index"
	"github.com/angeles725/niagara-go/internal/pycompat"
)

// freshnessAll is maintenance.cmd_freshness_all, with a last line counting each verdict.
// Its exit status is the worst verdict of an extracted version: 1 when one is STALE,
// 2 when one is UNKNOWN; a version not extracted yet never changes it.
func (e *Env) freshnessAll() error {
	reg, err := index.LoadRegistry(e.RepoRoot)
	if err != nil {
		return err
	}
	if reg == nil {
		fmt.Fprintln(e.Out, "No versions/registry.json; use plain 'freshness'.")
		return nil
	}
	fmt.Fprintf(e.Out, "FRESHNESS PER NIAGARA VERSION (default %s)\n", reg.Default)
	fmt.Fprintln(e.Out, strings.Repeat("=", 60))
	ids := reg.IDs()
	sort.Strings(ids)
	// Each version's status is computed concurrently, then printed in order.
	statuses := make([]string, len(ids))
	verdicts := make([]string, len(ids))
	errs := make([]error, len(ids))
	var wg sync.WaitGroup
	for i, vid := range ids {
		wg.Go(func() { statuses[i], verdicts[i], errs[i] = e.versionFreshness(reg, vid) })
	}
	wg.Wait()
	count := map[string]int{}
	for i, vid := range ids {
		if errs[i] != nil {
			return errs[i]
		}
		entry, _ := reg.Entry(vid)
		fmt.Fprintf(e.Out, "  %s %s %s\n", pycompat.LJust(vid, 12, ' '), pycompat.LJust(entry.DataRoot, 20, ' '), statuses[i])
		count[verdicts[i]]++
	}
	fmt.Fprintf(e.Out, "RESULT: %d FRESH, %d STALE, %d UNKNOWN, %d not extracted (exit 1 if any is STALE, "+
		"2 if any is UNKNOWN; a version not extracted does not count)\n",
		count[verdictFresh], count[verdictStale], count[verdictUnknown], count[verdictNotExtracted])
	switch {
	case count[verdictUnknown] > 0:
		return ExitCode(cli.ExitCannotSearch)
	case count[verdictStale] > 0:
		return ExitCode(cli.ExitNoMatch)
	}
	return nil
}

// The verdicts of freshness --all for one version.
const (
	verdictFresh        = "FRESH"
	verdictStale        = "STALE"
	verdictUnknown      = "UNKNOWN"
	verdictNotExtracted = "not extracted"
)

// versionFreshness is the status column of freshness --all for one version and its
// verdict: STALE when its indexes or its install's doc jars changed, UNKNOWN when its
// indexes cannot be compared with a baseline, else FRESH.
func (e *Env) versionFreshness(reg *index.Registry, vid string) (status, verdict string, err error) {
	entry, _ := reg.Entry(vid)
	root, err := index.DataRoot(e.RepoRoot, vid)
	if err != nil {
		return "", "", err
	}
	if !index.IsExtracted(root) {
		return fmt.Sprintf("NOT EXTRACTED (nh versions extract %s)", vid), verdictNotExtracted, nil
	}
	idx, err := indexStatus(root)
	if err != nil {
		return "", "", err
	}
	inst, err := installStatus(e.RepoRoot, vid, getStr(entry.Raw, "install_dir", ""))
	if err != nil {
		return "", "", err
	}
	switch {
	case strings.HasPrefix(idx, "STALE") || strings.HasPrefix(inst, "INSTALL CHANGED"):
		verdict = verdictStale
	case strings.HasPrefix(idx, "UNKNOWN"):
		verdict = verdictUnknown
	default:
		verdict = verdictFresh
	}
	return idx + "; " + inst, verdict, nil
}
