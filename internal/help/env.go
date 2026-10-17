package help

import (
	"fmt"
	"io"
	"os"

	"github.com/angeles725/niagara-go/internal/index"
)

// Env is what every command runs with (registry.Context plus the state
// app.py keeps in module globals): the output streams, the repository root,
// the selected version and its data root, and the miss counter that turns
// the exit status into 1.
type Env struct {
	In       io.Reader
	Out, Err io.Writer

	RepoRoot  string // the niagara-help checkout
	BaseDir   string // data root of the selected version ("" for versions)
	VersionID string // registry id of the selected version ("" without a registry)

	misses  int
	indexes map[string]*index.Object // loaded indexes/<name>, read-only (index_cache.py)
}

// Miss records that a lookup searched and matched nothing (scope.report_miss).
func (e *Env) Miss() { e.misses++ }

// ExitCode is an explicit exit status chosen by a command (Python
// sys.exit(n) inside a run function).
type ExitCode int

func (c ExitCode) Error() string { return fmt.Sprintf("exit status %d", int(c)) }

// MissingIndexError is index_cache.MissingIndexError: a required index file
// is absent, so the lookup cannot search (exit 2 with "ERROR: ...").
type MissingIndexError struct{ Name, BaseDir string }

func (e *MissingIndexError) Error() string {
	return fmt.Sprintf("index indexes/%s is missing under %s; rebuild the indexes "+
		"with: python3 tools/niagara_help.py rebuild-all", e.Name, e.BaseDir)
}

// loadIndex returns indexes/<name> of the selected data root, parsed once per
// Env (index_cache.load_index). A missing file is an empty object, or a
// MissingIndexError when required. The result is shared: never modify it.
func (e *Env) loadIndex(name string, required bool) (*index.Object, error) {
	if o, ok := e.indexes[name]; ok {
		return o, nil
	}
	path := index.Path(e.BaseDir, name)
	if _, err := os.Stat(path); err != nil {
		if required {
			return nil, &MissingIndexError{Name: name, BaseDir: e.BaseDir}
		}
		return index.NewObject(), nil
	}
	o, err := index.LoadObject(path)
	if err != nil {
		return nil, err
	}
	if e.indexes == nil {
		e.indexes = map[string]*index.Object{}
	}
	e.indexes[name] = o
	return o, nil
}

// notPortedError reports a command form that has no Go port yet (exit 2).
type notPortedError string

func (n notPortedError) Error() string { return "nh: '" + string(n) + "' is not ported yet" }
