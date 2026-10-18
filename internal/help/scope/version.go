package scope

import (
	"errors"
	"fmt"

	"github.com/angeles725/niagara-go/internal/index"
)

// NotExtractedError is a registered version whose data root has no indexes
// yet (app.py resolve_base_dir).
type NotExtractedError struct{ ID, Root string }

func (e *NotExtractedError) Error() string {
	return fmt.Sprintf("Niagara version %s is registered but not extracted at %s.", e.ID, e.Root)
}

// Resolve is app.py resolve_base_dir: index.Resolve, then a registered
// version must be extracted (*NotExtractedError). The error of an unknown or
// ambiguous version is *index.UnknownVersionError.
func Resolve(repo, requested string, getenv func(string) string) (id, root string, err error) {
	id, root, err = index.Resolve(repo, requested, getenv)
	if err != nil {
		return "", "", err
	}
	if id != "" && !index.IsExtracted(root) {
		return "", "", &NotExtractedError{ID: id, Root: root}
	}
	return id, root, nil
}

// VersionLines is what nh prints on stdout, then exits 2, when the version
// cannot be resolved: the ERROR line, plus the extract command for a version
// that is registered but not extracted.
func VersionLines(err error) []string {
	lines := []string{ErrorLine(err)}
	var ne *NotExtractedError
	if errors.As(err, &ne) {
		lines = append(lines, "Run: python3 tools/extract_version.py "+ne.ID)
	}
	return lines
}

// ErrorLine is the "ERROR: <err>" line nh prints on stderr, then exits 2, for
// a lookup that cannot search (a missing required index, a full-text index
// that cannot be built, a query without a searchable word).
func ErrorLine(err error) string { return "ERROR: " + err.Error() }
