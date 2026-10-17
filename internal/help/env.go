package help

import (
	"fmt"
	"io"
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

	misses int
}

// Miss records that a lookup searched and matched nothing (scope.report_miss).
func (e *Env) Miss() { e.misses++ }

// ExitCode is an explicit exit status chosen by a command (Python
// sys.exit(n) inside a run function).
type ExitCode int

func (c ExitCode) Error() string { return fmt.Sprintf("exit status %d", int(c)) }
