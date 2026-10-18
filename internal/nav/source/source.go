package source

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/angeles725/niagara-go/internal/argparse"
	"github.com/angeles725/niagara-go/internal/index"
	"github.com/angeles725/niagara-go/internal/nav/core"
	"github.com/angeles725/niagara-go/internal/nav/lib/codesearch"
	"github.com/angeles725/niagara-go/internal/nav/registry"
	"github.com/angeles725/niagara-go/internal/pycompat"
	"github.com/angeles725/niagara-go/internal/pyre"
	"github.com/angeles725/niagara-go/internal/textfold"
)

// runSource is commands/source.py run_source: one class, a --batch of them,
// or the usage lines.
func runSource(env *registry.Env, ns *argparse.Namespace) int {
	code, grep := ns.Bool("code"), ns.String("src_grep")
	switch batch, name := ns.String("src_batch"), ns.String("name"); {
	case batch != "":
		sourceBatch(env, batch, code, grep)
	case name != "":
		sourceClass(env, name, code, grep)
	default:
		fmt.Fprintln(env.Stdio.Out, "  Usage: source <class> [--code] [--grep pat]")
		fmt.Fprintln(env.Stdio.Out, "         source --batch A,B,C [--code] [--grep pat]")
	}
	return env.ExitCode()
}

// sourceBatch is grep_search.cmd_source_batch: sourceClass for each name of
// the CSV, each under a "=== name ===" separator; a miss does not stop it.
func sourceBatch(env *registry.Env, csv string, code bool, grep string) {
	var names []string
	for _, n := range strings.Split(csv, ",") {
		if n = pycompat.Strip(n); n != "" {
			names = append(names, n)
		}
	}
	out := env.Stdio.Out
	if len(names) == 0 {
		fmt.Fprintln(out, "  Usage: source --batch A,B,C [--code] [--grep pattern]")
		return
	}
	if code && len(names) > 1 {
		fmt.Fprint(env.Stdio.Err, "  WARN: --batch with --code can emit large output; "+
			"redirect to a file (e.g. `> out.txt`) if needed.\n")
	}
	for _, n := range names {
		fmt.Fprintf(out, "\n=== %s ===\n", n)
		sourceClass(env, n, code, grep)
	}
}

// resolveClass is grep_search._resolve_class: the index name and entry of a
// simple name (the first top-level entry, else the first one), of an FQN (the
// entry in that package, top-level first) or of a name differing only in
// case; ok=false when none matches. An entry without outer_class counts as
// top-level (Python raises KeyError there).
func resolveClass(classes *index.Object, name string) (string, *index.Object, bool) {
	topLevel := func(e *index.Object) bool { v, _ := e.Get("outer_class"); return v == nil }
	pick := func(k string) (string, *index.Object, bool) {
		entries := core.Objects(func() any { v, _ := classes.Get(k); return v }())
		for _, e := range entries {
			if topLevel(e) {
				return k, e, true
			}
		}
		if len(entries) > 0 {
			return k, entries[0], true
		}
		return "", nil, false
	}
	if i := strings.LastIndex(name, "."); i >= 0 {
		pkg, simple := name[:i], name[i+1:]
		v, _ := classes.Get(simple)
		entries := core.Objects(v)
		inPkg := func(e *index.Object) bool { p, _ := e.Get("package"); s, ok := p.(string); return ok && s == pkg }
		for _, e := range entries {
			if inPkg(e) && topLevel(e) {
				return simple, e, true
			}
		}
		for _, e := range entries {
			if inPkg(e) {
				return simple, e, true
			}
		}
		return "", nil, false
	}
	if classes.Has(name) {
		return pick(name)
	}
	lower := textfold.Lower(name)
	for _, k := range classes.Keys() {
		if textfold.Lower(k) == lower {
			return pick(k)
		}
	}
	return "", nil, false
}

// sourceClass is grep_search.cmd_source: the class's index entry (info
// mode), its source with line numbers (--code) or its lines matching a regex
// (--grep, which wins over --code).
func sourceClass(env *registry.Env, name string, code bool, grep string) {
	data := env.LoadClassIndex()
	if data == nil {
		return
	}
	classes := data.Object("classes")
	if classes == nil {
		classes = index.NewObject()
	}
	resolved, entry, ok := resolveClass(classes, name)
	if !ok {
		env.ClassMiss(name, data, "", fmt.Sprintf("search '*%s*'", name))
		return
	}
	// The corpus check runs in every mode (an absent corpus is an exit-2 error
	// even when the info block still prints).
	root := env.SourceRoot(data.Object("_meta"))
	if root == "" && (code || grep != "") {
		return
	}
	out := env.Stdio.Out
	path := "None"
	if root != "" {
		path = pycompat.PosixJoin(root, core.Str(entry, "path"))
	}
	if !code && grep == "" {
		fmt.Fprintf(out, "\n  Class:    %s\n", resolved)
		fmt.Fprintf(out, "  Package:  %s\n", core.Str(entry, "package"))
		fmt.Fprintf(out, "  Module:   %s\n", core.Str(entry, "module"))
		fmt.Fprintf(out, "  Kind:     %s\n", core.Str(entry, "kind"))
		fmt.Fprintf(out, "  Lines:    %s\n", core.Commas(core.Int(entry, "lines")))
		fmt.Fprintf(out, "  Path:     %s\n", core.Str(entry, "path"))
		if root != "" && !isFile(path) {
			fmt.Fprintln(out, "  WARNING:  Source file not found on disk!")
		}
		fmt.Fprintln(out)
		return
	}
	if !isFile(path) {
		fmt.Fprintf(out, "  ERROR: Source file not found: %s\n", path)
		return
	}
	lines, err := codesearch.ReadLines(path)
	if err != nil {
		fmt.Fprintf(out, "  ERROR: Cannot read source: %s\n", index.OSError(err, path))
		return
	}
	module := core.Str(entry, "module")
	if grep != "" {
		sourceGrep(out, lines, resolved, module, grep)
		return
	}
	fmt.Fprintf(out, "\n  SOURCE: %s (%s, %d lines)\n", resolved, module, len(lines))
	fmt.Fprintf(out, "  PATH:   %s\n\n", core.Str(entry, "path"))
	for i, line := range lines {
		fmt.Fprintf(out, "%4d | %s\n", i+1, codesearch.RStrip(line))
	}
	fmt.Fprintln(out)
}

// sourceGrep is the --grep mode of cmd_source: an invalid regex is reported
// on stdout without changing the exit status, as in Python.
func sourceGrep(out io.Writer, lines []string, class, module, pattern string) {
	re, err := pyre.Compile(pattern, 0)
	if err != nil {
		fmt.Fprintf(out, "  ERROR: Invalid regex: %v\n", err)
		return
	}
	fmt.Fprintf(out, "\n  source %s --grep /%s/ (%s)\n\n", class, pattern, module)
	matches := 0
	for i, line := range lines {
		if re.MatchString(line) {
			matches++
			fmt.Fprintf(out, "  %4d | %s\n", i+1, codesearch.RStrip(line))
		}
	}
	if matches == 0 {
		fmt.Fprintf(out, "  No matches for /%s/ in %s (%d lines).\n", pattern, class, len(lines))
	} else {
		fmt.Fprintf(out, "\n  %d matches in %d lines\n", matches, len(lines))
	}
	fmt.Fprintln(out)
}

func isFile(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.Mode().IsRegular()
}
