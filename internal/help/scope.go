package help

// Honest-zero reporting (scope.py): when a lookup matches nothing it prints
// the scope it searched, what that scope excludes on purpose and the nearest
// alternative commands (the text is the scope library's), and counts a miss so
// the exit status becomes 1.

import (
	"fmt"

	"github.com/angeles725/niagara-go/internal/help/scope"
	"github.com/angeles725/niagara-go/internal/index"
)

// reportMiss prints the honest-zero block for a lookup that matched nothing
// in the named scope and counts the miss (scope.report_miss).
func (e *Env) reportMiss(command, query, name string) error {
	e.Miss()
	lines, err := scope.Miss(e.BaseDir, command, query, name, e.optionalIndex)
	if err != nil {
		return err
	}
	for _, l := range lines {
		fmt.Fprintln(e.Out, l)
	}
	return nil
}

// optionalIndex is the scope.Loader of the selected data root: the Env cache.
func (e *Env) optionalIndex(name string) (*index.Object, error) { return e.loadIndex(name, false) }
