// Package cli holds the conventions every command shares: exit codes,
// injectable stdio and the process entry point.
package cli

import (
	"io"
	"os"
)

// Exit codes mean the same thing in every tool (TOOLING-GUIDELINES principle 2).
const (
	ExitFound        = 0 // found
	ExitNoMatch      = 1 // searched, no match (scope and counts printed)
	ExitCannotSearch = 2 // missing or corrupt index, bad input, unreadable source
)

// Stdio is the process I/O a command writes to; tests pass buffers.
type Stdio struct {
	In  io.Reader
	Out io.Writer
	Err io.Writer
}

// Main is the signature every command package exports.
type Main func(args []string, s Stdio) int

// Run executes m with the process arguments and exits with its code.
func Run(m Main) {
	os.Exit(m(os.Args[1:], Stdio{In: os.Stdin, Out: os.Stdout, Err: os.Stderr}))
}
