package searchdoor

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/angeles725/niagara-go/internal/pycompat"
	"github.com/angeles725/niagara-go/internal/pyre"
)

// source is the backend a call goes to: the blocks (corpus-nav), docs (niagara_help) or
// code (module_nav) source.
type source int

const (
	blocksSource source = iota + 1
	docsSource
	codeSource
)

// Backend runs one backend call and types its answer. src picks the backend; argv[0] is
// its script path (it locates the backend's checkout and names it in messages), the rest
// its arguments; timeout is in seconds.
type Backend interface {
	Run(src source, argv []string, timeout float64) Raw
}

// MissLine is a backend's miss/scope line at the start of a line: corpus-nav
// "(no matches; searched ...)", niagara_help "No match for ... in this scope.",
// module_nav "0 matches for /rx/ in N .java files" (backend.MISS_LINE).
var MissLine = pyre.MustCompile(`^\s*\(?\s*(?:no match(?:es)?\b|0 matches\b)`, pyre.IGNORECASE|pyre.MULTILINE)

// Classify types one finished backend run (backend.run_tool after the process
// exits): the exit code alone is not trusted. "empty" needs exit 1 AND a miss
// line; "ok" needs exit 0 AND output that does not open with a miss line;
// anything else is UNAVAILABLE with the reason.
func Classify(rc int, stdout, stderr string, ms int) Raw {
	out := strings.TrimRight(stdout, "\n")
	errLines := pycompat.SplitLines(pycompat.Strip(stderr), false)
	if rc != 0 && rc != 1 {
		why := ""
		switch {
		case len(errLines) > 0:
			why = errLines[len(errLines)-1]
		default:
			if outLines := pycompat.SplitLines(pycompat.Strip(out), false); len(outLines) > 0 {
				why = outLines[0]
			} else {
				why = "exit " + strconv.Itoa(rc)
			}
		}
		return Raw{Unavailable, out, fmt.Sprintf("error: %s (exit %d)", pycompat.Strip(why), rc), ms, rc, true}
	}
	if rc == 1 {
		if MissLine.MatchString(out) {
			return Raw{State: Empty, Out: out, MS: ms, RC: 1, HasRC: true}
		}
		why := "no output"
		if len(errLines) > 0 {
			why = errLines[len(errLines)-1]
		}
		return Raw{Unavailable, out, "error: exit 1 without a miss/scope line (crashed?): " + why, ms, 1, true}
	}
	var lines []string
	for _, ln := range pycompat.SplitLines(out, false) {
		if pycompat.Strip(ln) != "" {
			lines = append(lines, ln)
		}
	}
	if len(lines) == 0 {
		return Raw{Unavailable, out, "ambiguous: exit 0 with no output (neither rows nor a miss/scope line)", ms, 0, true}
	}
	if MissLine.MatchString(lines[0]) {
		return Raw{Unavailable, out, "ambiguous: exit 0 but the output is a miss line: " + pycompat.Strip(lines[0]), ms, 0, true}
	}
	return Raw{State: OK, Out: out, MS: ms, RC: 0, HasRC: true}
}

// formatG3 is Python's f"{x:.3g}".
func formatG3(x float64) string { return strconv.FormatFloat(x, 'g', 3, 64) }

// textOf decodes like subprocess text mode: UTF-8 with errors="replace" and
// universal newlines ("\r\n" and "\r" read as "\n").
func textOf(b []byte) string {
	return pycompat.UniversalNewlines(pycompat.DecodeReplace(b))
}

// pyPath is str(pathlib.PurePosixPath(p)): repeated slashes and "." parts
// dropped, no trailing slash, ".." kept.
func pyPath(p string) string {
	if p == "" {
		return "."
	}
	lead := ""
	switch {
	case strings.HasPrefix(p, "//") && !strings.HasPrefix(p, "///"):
		lead = "//"
	case strings.HasPrefix(p, "/"):
		lead = "/"
	}
	var parts []string
	for _, part := range strings.Split(p, "/") {
		if part != "" && part != "." {
			parts = append(parts, part)
		}
	}
	if len(parts) == 0 {
		if lead != "" {
			return lead
		}
		return "."
	}
	return lead + strings.Join(parts, "/")
}

// nonblank is backend.nonblank: the right-stripped lines that are not blank.
func nonblank(text string) []string {
	var out []string
	for _, ln := range pycompat.SplitLines(text, false) {
		if pycompat.Strip(ln) != "" {
			out = append(out, strings.TrimRightFunc(ln, pycompat.IsSpace))
		}
	}
	return out
}

// strippedNonblank is [ln.strip() for ln in nonblank(text)][:n] (n < 0: all).
func strippedNonblank(text string, n int) []string {
	var out []string
	for _, ln := range nonblank(text) {
		if n >= 0 && len(out) == n {
			break
		}
		out = append(out, pycompat.Strip(ln))
	}
	return out
}
