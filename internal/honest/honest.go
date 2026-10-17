// Package honest is tools/lib/honest.py: the "honest zero" a lookup prints
// when it finds nothing, saying WHAT it searched and HOW MUCH, so a miss is
// never read as "does not exist". An unreadable or missing input is an error
// with a non-zero exit, never an empty result.
package honest

import (
	"fmt"
	"io"
	"strings"

	"github.com/angeles725/niagara-go/internal/cli"
)

// ExitInput is honest.EXIT_INPUT: missing, unreadable or invalid input.
const ExitInput = cli.ExitCannotSearch

// Count is one counted thing of a scope; underscores in Label print as
// spaces (Python's keyword names: java_files=3 -> "3 java files").
type Count struct {
	Label string
	N     int
}

// Scope is honest.scope: "searched <where>: 12 components, 3 links", counts in order.
func Scope(where string, counts ...Count) string {
	if len(counts) == 0 {
		return "searched " + where
	}
	parts := make([]string, len(counts))
	for i, c := range counts {
		parts[i] = fmt.Sprintf("%d %s", c.N, strings.ReplaceAll(c.Label, "_", " "))
	}
	return "searched " + where + ": " + strings.Join(parts, ", ")
}

// Miss is honest.miss: "(no <what>; searched <where>: <counts>)".
func Miss(what, where string, counts ...Count) string {
	return "(no " + what + "; " + Scope(where, counts...) + ")"
}

// Fail is honest.fail without the process exit: it writes "<tool>: <message>"
// to stderr and returns code, for a Main to return (D2: commands return their
// exit code, cli.Run exits).
func Fail(stderr io.Writer, tool, message string, code int) int {
	fmt.Fprintf(stderr, "%s: %s\n", tool, message)
	return code
}
