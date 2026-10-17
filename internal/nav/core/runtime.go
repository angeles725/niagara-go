// Package core is the module-nav runtime every command uses (module_nav_lib
// outcome, indexes, corpus and suggest): honest outcomes and exit codes, the
// index loader with its stderr progress lines, the organized/ corpus walkers and
// did-you-mean suggestions. It imports no command group.
package core

import (
	"database/sql"
	"fmt"
	"io"

	"github.com/angeles725/niagara-go/internal/index"
)

// Outcome statuses (outcome.py): the highest one raised decides the exit code.
const (
	StatusMiss  = 1 // searched, no single answer
	StatusError = 2 // could not search
)

// Runtime is one module-nav process (or one REPL session): where it writes, the
// exit status raised so far and the indexes already loaded.
type Runtime struct {
	BaseDir string    // the module-navigator directory holding indexes/
	Out     io.Writer // stdout
	Err     io.Writer // stderr

	status int
	json   map[string]*index.Object
	db     map[string]*sql.DB
	noted  map[string]bool // optional indexes already reported absent
}

// New returns the runtime of a command run against baseDir.
func New(baseDir string, out, errw io.Writer) *Runtime {
	return &Runtime{BaseDir: baseDir, Out: out, Err: errw,
		json: map[string]*index.Object{}, db: map[string]*sql.DB{}, noted: map[string]bool{}}
}

func (r *Runtime) raise(code int) { r.status = max(r.status, code) }

// Miss is outcome.miss: a lookup with no result over scope, a counted
// description of what was searched, and an optional hint. Exit status 1.
func (r *Runtime) Miss(subject, scope, hint string) {
	fmt.Fprintf(r.Out, "  0 matches for %s in %s.\n", subject, scope)
	if hint != "" {
		fmt.Fprintf(r.Out, "  Try: %s\n", hint)
	}
	r.raise(StatusMiss)
}

// Ambiguous is outcome.ambiguous: the caller printed several candidates and
// picked none. Exit status 1.
func (r *Runtime) Ambiguous() { r.raise(StatusMiss) }

// Error is outcome.error: a failure to search at all, on stderr. Exit status 2.
func (r *Runtime) Error(message, hint string) {
	fmt.Fprintf(r.Err, "ERROR: %s\n", message)
	if hint != "" {
		fmt.Fprintf(r.Err, "  %s\n", hint)
	}
	r.raise(StatusError)
}

// Unreadable is the CLI's last resort for a corrupt index met inside a command
// (commands/__init__.py main catching JSONDecodeError and sqlite3.DatabaseError).
func (r *Runtime) Unreadable(err error) {
	r.Error(fmt.Sprintf("unreadable index: %v", err), "Run: ./reindex.sh")
}

// ExitCode is outcome.exit_code: the highest status raised.
func (r *Runtime) ExitCode() int { return r.status }

// Reset is outcome.reset: forget earlier outcomes (one REPL command at a time).
func (r *Runtime) Reset() { r.status = 0 }

// Built is outcome.built: "index built <timestamp>" from an index _meta block.
func Built(meta *index.Object) string {
	if meta != nil {
		if v, ok := meta.Get("timestamp"); ok {
			return "index built " + PyStr(v)
		}
	}
	return "index built at an unknown time"
}

// Close releases the SQLite connections the runtime opened.
func (r *Runtime) Close() {
	for _, db := range r.db {
		db.Close()
	}
}
