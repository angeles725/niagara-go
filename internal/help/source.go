package help

// Java source lookups over source-index.json and source/ (source_search.py):
// source (lookup, glob listing, --code, --grep). The resolvers source_matches and
// corpus_path are shared with selftest --recall; source-grep is in source_grep.go.

import (
	"fmt"
	"sort"
	"strings"

	"github.com/angeles725/niagara-go/internal/fnmatch"
	"github.com/angeles725/niagara-go/internal/index"
	"github.com/angeles725/niagara-go/internal/pycompat"
	"github.com/angeles725/niagara-go/internal/pypath"
	"github.com/angeles725/niagara-go/internal/textfold"
)

// sourceMatch is one source-index entry: its key and its data.
type sourceMatch struct {
	key  string
	data *index.Object
}

// sourceMatches is source_search.source_matches: the plain key of a class
// plus the "Name (module)" keys colliding simple names get, in index order.
func sourceMatches(idx *index.Object, name string) []sourceMatch {
	var out []sourceMatch
	for _, k := range idx.Keys() {
		if k == name || strings.HasPrefix(k, name+" (") {
			out = append(out, sourceMatch{k, idx.Object(k)})
		}
	}
	return out
}

// corpusPath is source_search.corpus_path: the corpus-relative path of a
// source-index entry, os.path.normpath("source/" + path).
func corpusPath(data *index.Object) string {
	return pypath.PosixNormpath(pypath.PosixJoin("source", getStr(data, "path", "")))
}

// sourceLines reads a source file the way `for line in open(path, "r",
// encoding="utf-8", errors="replace")` iterates it: invalid UTF-8 replaced,
// universal newlines, one entry per line without its "\n".
func sourceLines(file string) ([]string, error) {
	text, err := pycompat.ReadTextReplace(file)
	if err != nil {
		return nil, err
	}
	if text == "" {
		return nil, nil
	}
	lines := strings.Split(text, "\n")
	if lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines, nil
}

// rstrip is str.rstrip().
func rstrip(s string) string { return strings.TrimRightFunc(s, textfold.IsSpace) }

// SourceParams are the arguments of `source`.
type SourceParams struct {
	Name string
	Code bool
	Grep string // a Python regex, matched case-insensitively against each line
}

// sourceGlob returns the sorted keys matching pattern with fnmatch, then
// case-insensitively.
func sourceGlob(idx *index.Object, pattern string) []string {
	var out []string
	for _, k := range idx.Keys() {
		if fnmatch.Fnmatch(k, pattern) {
			out = append(out, k)
		}
	}
	if len(out) == 0 {
		for _, k := range idx.Keys() {
			if lowerGlob(k, pattern) {
				out = append(out, k)
			}
		}
	}
	sort.Strings(out)
	return out
}

// Source is source_search.cmd_source: the source-index entry of a class
// (exact, then case-insensitive), a listing when the name is a glob, and
// with --code the numbered source text or with --grep the matching lines.
func (e *Env) Source(p SourceParams) error {
	idx, err := e.loadIndex("source-index.json", false)
	if err != nil {
		return err
	}
	name := p.Name
	if !idx.Has(name) {
		want := textfold.Lower(name)
		var fold []string
		for _, k := range idx.Keys() {
			if textfold.Lower(k) == want {
				fold = append(fold, k)
			}
		}
		if len(fold) == 0 {
			return e.printSourceGlob(idx, name)
		}
		name = fold[0]
	}
	data := idx.Object(name)
	fmt.Fprintf(e.Out, "SOURCE: %s\n", name)
	fmt.Fprintf(e.Out, "MODULE: %s\n", pyStr(get(data, "module", nil)))
	fmt.Fprintf(e.Out, "PACKAGE: %s\n", pyStr(get(data, "package", nil)))
	fmt.Fprintf(e.Out, "LINES: %s\n", pyStr(get(data, "lines", nil)))
	fmt.Fprintf(e.Out, "PATH: %s\n", corpusPath(data))
	var others []string
	for _, m := range sourceMatches(idx, strings.SplitN(name, " (", 2)[0]) {
		if m.key != name {
			others = append(others, m.key)
		}
	}
	if len(others) > 0 {
		fmt.Fprintf(e.Out, "ALSO: %s\n", strings.Join(others, ", "))
	}
	if p.Grep != "" {
		return e.grepSource(data, p.Grep)
	}
	if !p.Code {
		return nil
	}
	fmt.Fprintln(e.Out)
	file := pycompat.Join(e.BaseDir, "source", getStr(data, "path", ""))
	lines, err := sourceLines(file)
	if err != nil {
		fmt.Fprintf(e.Out, "Error reading source: %s\n", index.OSError(err, file))
		return nil
	}
	for i, line := range lines {
		fmt.Fprintf(e.Out, "%4d | %s\n", i+1, rstrip(line))
	}
	return nil
}

// printSourceGlob lists the source files a glob matches (at most 20), or
// reports the miss.
func (e *Env) printSourceGlob(idx *index.Object, pattern string) error {
	matches := sourceGlob(idx, pattern)
	if len(matches) == 0 {
		return e.reportMiss("source", pattern, "source")
	}
	fmt.Fprintf(e.Out, "%d source files matching '%s':\n\n", len(matches), pattern)
	for _, n := range pyHead(matches, 20) {
		d := idx.Object(n)
		fmt.Fprintf(e.Out, "  %s (%s, %s lines) - %s\n", n, pyStr(get(d, "module", nil)),
			pyStr(get(d, "lines", nil)), pyStr(get(d, "package", nil)))
	}
	if len(matches) > 20 {
		fmt.Fprintf(e.Out, "  ... and %d more\n", len(matches)-20)
	}
	return nil
}
