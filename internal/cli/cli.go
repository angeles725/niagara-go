// Package cli holds the conventions every command shares: exit codes,
// injectable stdio and the process entry point.
package cli

import (
	"errors"
	"io"
	"os"
)

// Exit codes mean the same thing in every tool (TOOLING-GUIDELINES principle 2).
const (
	ExitFound        = 0 // found
	ExitNoMatch      = 1 // searched, no match (scope and counts printed)
	ExitCannotSearch = 2 // missing or corrupt index, bad input, unreadable source
)

// Exit is a Python SystemExit carrying a message: the command prints Msg on
// stderr and exits with Code.
type Exit struct {
	Msg  string
	Code int
}

func (e *Exit) Error() string { return e.Msg }

// SystemExit is raise SystemExit(msg): the message on stderr, exit 1.
func SystemExit(msg string) error { return &Exit{Msg: msg, Code: 1} }

// ExitCode is the code of the Exit in err's chain; ok is false when there is none.
func ExitCode(err error) (code int, ok bool) {
	var e *Exit
	if errors.As(err, &e) {
		return e.Code, true
	}
	return 0, false
}

// Stdio is the process I/O a command writes to and the environment it reads;
// tests pass buffers and, to stay off the process environment (no os.Setenv, so
// they can run in parallel), a Getenv of their own.
type Stdio struct {
	In  io.Reader
	Out io.Writer
	Err io.Writer
	// Getenv reads one environment variable; nil reads the process (os.Getenv),
	// so a Stdio built without it behaves as before.
	Getenv func(name string) string
}

// Env is the value of the environment variable name ("" when unset), read
// through Getenv.
func (s Stdio) Env(name string) string {
	if s.Getenv == nil {
		return os.Getenv(name)
	}
	return s.Getenv(name)
}

// Main is the signature every command package exports.
type Main func(args []string, s Stdio) int

// Run executes m with the process arguments and exits with its code.
func Run(m Main) {
	os.Exit(m(os.Args[1:], processStdio()))
}

// processStdio is the process's own streams and environment.
func processStdio() Stdio {
	return Stdio{In: os.Stdin, Out: os.Stdout, Err: os.Stderr, Getenv: os.Getenv}
}
