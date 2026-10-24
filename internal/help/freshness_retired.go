package help

import (
	"fmt"
	"os"
	"strings"

	"github.com/angeles725/niagara-go/internal/help/fts"
	"github.com/angeles725/niagara-go/internal/index"
)

// retiredLeft lists the retired index files (fts.RetiredIndexes: freshness names one
// left on disk so it can be deleted by hand, and never deletes it) under root/indexes/.
func retiredLeft(root string) []string {
	var out []string
	for _, name := range fts.RetiredIndexes {
		if st, err := os.Stat(index.Path(root, name)); err == nil && st.Mode().IsRegular() {
			out = append(out, name)
		}
	}
	return out
}

// retiredLines prints the retired index files left on disk, if any.
func (e *Env) retiredLines() {
	left := retiredLeft(e.BaseDir)
	if len(left) == 0 {
		return
	}
	fmt.Fprintln(e.Out, "Retired index files still on disk (nh neither reads nor rebuilds them; delete by hand):")
	for _, name := range left {
		fmt.Fprintf(e.Out, "  indexes/%s\n", name)
	}
}

// retiredSuffix is the freshness --all note of the retired index files left in root.
func retiredSuffix(root string) string {
	if left := retiredLeft(root); len(left) > 0 {
		return "; retired index left: " + strings.Join(left, ", ")
	}
	return ""
}

// indexStatus is the index column of freshness --all: indexState, then the
// retired index files left in root.
func indexStatus(root string) (string, error) {
	s, err := indexState(root)
	if err != nil {
		return "", err
	}
	return s + retiredSuffix(root), nil
}
