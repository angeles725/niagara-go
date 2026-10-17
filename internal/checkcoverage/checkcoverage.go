// Package checkcoverage is tools/check-coverage.py, the prior-coverage gate run before a
// new research focus opens: for each module or package name it lists the blocks whose
// H1 names it (SUBJECT) or whose body mentions it, over every block file of the corpus.
//
// Exit: 0 every name clear, 2 prior coverage found, 1 usage (no name), 3 no block file
// found (the gate could not run) or, intended difference from Python (which skips them
// silently), a block file that could not be read when no name is covered: each one is
// named on stderr and "clear" is reported as incomplete, since it is not proven. The repository is the parent of the directory holding
// the executable, as invoked and without resolving symlinks (Python:
// os.path.dirname(os.path.dirname(os.path.abspath(__file__)))).
package checkcoverage

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/angeles725/niagara-go/internal/blockfiles"
	"github.com/angeles725/niagara-go/internal/cli"
	"github.com/angeles725/niagara-go/internal/pycompat"
	"github.com/angeles725/niagara-go/internal/textfold"
)

// repoRoot locates the repository; tests replace it.
var repoRoot = func() (string, error) {
	exe, err := cli.InvokedPath()
	if err != nil {
		return "", err
	}
	return filepath.Dir(filepath.Dir(exe)), nil
}

type hit struct{ block, h1 string }

type coverage struct{ subject, body []hit }

// Main runs check-coverage with argv (without the program name).
func Main(args []string, s cli.Stdio) int {
	var names []string
	asJSON, help := false, false
	for _, a := range args {
		switch {
		case !strings.HasPrefix(a, "-"):
			names = append(names, a)
		case a == "--json":
			asJSON = true
		case a == "-h" || a == "--help":
			help = true
		}
	}
	if len(names) == 0 || help {
		fmt.Fprint(s.Out, helpText+"\n") // print(__doc__): the docstring ends with a newline too
		if help {
			return 0
		}
		return 1
	}
	root, err := repoRoot()
	if err != nil {
		fmt.Fprintf(s.Err, "check-coverage: cannot locate the repository: %v\n", err)
		return 3
	}
	results, order, nfiles, unreadable := scan(root, names)
	for _, u := range unreadable {
		fmt.Fprintf(s.Err, "check-coverage: cannot read %s: %s\n", u.path, u.reason)
	}
	if nfiles == 0 {
		fmt.Fprintf(s.Err, "check-coverage: no *bloque*.md block files under %s -- gate not run\n", root)
		return 3
	}
	covered := false
	for _, n := range names {
		if c := results[n]; len(c.subject) > 0 || len(c.body) > 0 {
			covered = true
		}
	}
	if asJSON {
		fmt.Fprintln(s.Out, jsonReport(results, order, nfiles, len(unreadable), covered))
		switch {
		case covered:
			return 2
		case len(unreadable) > 0:
			return 3
		}
		return 0
	}
	report(s, names, results, nfiles, len(unreadable))
	if covered {
		fmt.Fprintln(s.Out, `VERDICT: prior coverage found. DO NOT declare "extends X, never re-derives" blindly.`)
		fmt.Fprintln(s.Out, "For each covered module, decide in the bootstrap: net-new / deepening of B<n> (cite it) /")
		fmt.Fprintln(s.Out, "already covered -> out of scope. A focus that is entirely covered does NOT open.")
		return 2
	}
	if len(unreadable) > 0 {
		fmt.Fprintf(s.Out, "VERDICT: incomplete. %d block file(s) could not be read (named on stderr), "+
			"so \"clear\" is not proven.\n", len(unreadable))
		return 3
	}
	fmt.Fprintln(s.Out, "VERDICT: clear. No prior coverage for any name — safe to open as net-new.")
	return 0
}

func report(s cli.Stdio, names []string, results map[string]*coverage, nfiles, unreadable int) {
	if unreadable > 0 {
		fmt.Fprintf(s.Out, "prior-coverage gate — scanned %d block(s), %d unreadable\n\n", nfiles, unreadable)
	} else {
		fmt.Fprintf(s.Out, "prior-coverage gate — scanned %d block(s)\n\n", nfiles)
	}
	for _, n := range names {
		c := results[n]
		if len(c.subject) == 0 && len(c.body) == 0 {
			fmt.Fprintf(s.Out, "  [CLEAR]   %s — no prior block mentions it. Safe to open as net-new.\n", n)
			continue
		}
		tag := "MENTIONED"
		if len(c.subject) > 0 {
			tag = "COVERED"
		}
		fmt.Fprintf(s.Out, "  [%s] %s:\n", tag, n)
		for _, e := range c.subject {
			fmt.Fprintf(s.Out, "      SUBJECT  %s — %s\n", e.block, e.h1)
		}
		for _, e := range c.body[:min(len(c.body), 12)] {
			fmt.Fprintf(s.Out, "      body     %s — %s\n", e.block, e.h1)
		}
		if len(c.body) > 12 {
			fmt.Fprintf(s.Out, "      body     … +%d more block(s)\n", len(c.body)-12)
		}
		fmt.Fprintln(s.Out)
	}
}

// scan is check-coverage.scan: one pass over every block file, each name classified as
// SUBJECT (in the H1) or body per block. A repeated name shares one result, so its hits
// are recorded once per repetition, as in Python. order is the distinct names in order;
// unreadable lists the block files that could not be read, in scan order.
func scan(root string, names []string) (map[string]*coverage, []string, int, []unreadableFile) {
	results := map[string]*coverage{}
	var order []string
	for _, n := range names {
		if results[n] == nil {
			results[n] = &coverage{}
			order = append(order, n)
		}
	}
	var unreadable []unreadableFile
	files := blockfiles.AllBlocks(root)
	for _, b := range files {
		ident := "B" + b.Label
		if b.Corpus != "main" {
			if rel, err := filepath.Rel(root, b.Path); err == nil {
				ident = rel
			}
		}
		raw, err := os.ReadFile(b.Path)
		if err != nil {
			unreadable = append(unreadable, unreadableFile{b.Path, strerror(err)})
			continue
		}
		text := universalNewlines(pycompat.DecodeReplace(raw))
		low := textfold.Lower(text)
		h1, h1Done := "", false
		for _, n := range names {
			lowName := textfold.Lower(n)
			if !strings.Contains(low, lowName) {
				continue
			}
			if !h1Done {
				h1, h1Done = firstH1(text), true
			}
			e := hit{ident, h1}
			if strings.Contains(textfold.Lower(h1), lowName) {
				results[n].subject = append(results[n].subject, e)
			} else {
				results[n].body = append(results[n].body, e)
			}
		}
	}
	return results, order, len(files), unreadable
}

// unreadableFile is a block file the scan could not read and the OS error text.
type unreadableFile struct{ path, reason string }

// strerror renders an I/O error the way Python's OSError.strerror does.
func strerror(err error) string {
	var errno syscall.Errno
	if errors.As(err, &errno) {
		if msg := errno.Error(); msg != "" {
			return strings.ToUpper(msg[:1]) + msg[1:]
		}
	}
	return err.Error()
}

// firstH1 is check-coverage.first_h1: the first stripped "# " line without its marker.
func firstH1(text string) string {
	for _, line := range strings.SplitAfter(text, "\n") {
		if line = pycompat.Strip(line); strings.HasPrefix(line, "# ") {
			return pycompat.Strip(line[2:])
		}
	}
	return ""
}

func universalNewlines(s string) string {
	return strings.ReplaceAll(strings.ReplaceAll(s, "\r\n", "\n"), "\r", "\n")
}
