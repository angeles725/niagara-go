// Package checkcoverage is tools/check-coverage.py, the prior-coverage gate run before a
// new research focus opens: for each module or package name it lists the blocks whose
// H1 names it (SUBJECT) or whose body mentions it, over every block file of the corpus.
//
// Exit: 0 every name clear, 2 prior coverage found, 1 usage (no name), 3 no block file
// found (the gate could not run). The repository is the parent of the directory holding
// the executable, as invoked and without resolving symlinks (Python:
// os.path.dirname(os.path.dirname(os.path.abspath(__file__)))).
package checkcoverage

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/angeles725/niagara-go/internal/cli"
	"github.com/angeles725/niagara-go/internal/pycompat"
	"github.com/angeles725/niagara-go/internal/textfold"
)

// repoRoot locates the repository; tests replace it.
var repoRoot = func() (string, error) {
	exe, err := invokedPath()
	if err != nil {
		return "", err
	}
	return filepath.Dir(filepath.Dir(exe)), nil
}

// invokedPath is the executable as it was started (Python's __file__): argv[0] made
// absolute, or looked up on $PATH when it has no separator; else os.Executable.
func invokedPath() (string, error) {
	arg0 := os.Args[0]
	if !strings.ContainsRune(arg0, filepath.Separator) && !strings.ContainsRune(arg0, '/') {
		found, err := lookPath(arg0)
		if err != nil {
			return os.Executable()
		}
		arg0 = found
	}
	return filepath.Abs(arg0)
}

func lookPath(name string) (string, error) {
	for _, dir := range filepath.SplitList(os.Getenv("PATH")) {
		if dir == "" {
			dir = "."
		}
		p := filepath.Join(dir, name)
		if st, err := os.Stat(p); err == nil && !st.IsDir() && st.Mode()&0o111 != 0 {
			return p, nil
		}
	}
	return "", fs.ErrNotExist
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
	results, order, nfiles := scan(root, names)
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
		fmt.Fprintln(s.Out, jsonReport(results, order, nfiles, covered))
		if covered {
			return 2
		}
		return 0
	}
	report(s, names, results, nfiles)
	if covered {
		fmt.Fprintln(s.Out, `VERDICT: prior coverage found. DO NOT declare "extends X, never re-derives" blindly.`)
		fmt.Fprintln(s.Out, "For each covered module, decide in the bootstrap: net-new / deepening of B<n> (cite it) /")
		fmt.Fprintln(s.Out, "already covered -> out of scope. A focus that is entirely covered does NOT open.")
		return 2
	}
	fmt.Fprintln(s.Out, "VERDICT: clear. No prior coverage for any name — safe to open as net-new.")
	return 0
}

func report(s cli.Stdio, names []string, results map[string]*coverage, nfiles int) {
	fmt.Fprintf(s.Out, "prior-coverage gate — scanned %d block(s)\n\n", nfiles)
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
// are recorded once per repetition, as in Python. order is the distinct names in order.
func scan(root string, names []string) (map[string]*coverage, []string, int) {
	results := map[string]*coverage{}
	var order []string
	for _, n := range names {
		if results[n] == nil {
			results[n] = &coverage{}
			order = append(order, n)
		}
	}
	files := allBlocks(root)
	for _, b := range files {
		ident := "B" + b.label
		if b.corpus != "main" {
			if rel, err := filepath.Rel(root, b.path); err == nil {
				ident = rel
			}
		}
		raw, err := os.ReadFile(b.path)
		if err != nil {
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
	return results, order, len(files)
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
