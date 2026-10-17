package classrank

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/angeles725/niagara-go/internal/index"
	"github.com/angeles725/niagara-go/internal/nav/core"
	"github.com/angeles725/niagara-go/internal/nav/registry"
)

const (
	classIndex = "class-index.json"
	xrefIndex  = "xref-index.json"
	progressMB = 20 // indexes this large print a progress line (indexes.py)
)

func absIndex(env *registry.Env, name string) string {
	p := core.IndexPath(env.BaseDir, name)
	if a, err := filepath.Abs(p); err == nil {
		return a
	}
	return p
}

// progressStart is _progress_start: the progress line of a large index.
func progressStart(env *registry.Env, path, name string) bool {
	st, err := os.Stat(path)
	if err != nil || !st.Mode().IsRegular() {
		return false
	}
	mb := float64(st.Size()) / (1024 * 1024)
	if mb < progressMB {
		return false
	}
	fmt.Fprintf(env.Err, "  Loading %s (%s MB)...", name, core.Fixed(mb, 1))
	return true
}

func progressEnd(env *registry.Env, started, ok bool) {
	if !started {
		return
	}
	if ok {
		fmt.Fprint(env.Err, " OK\n")
	} else {
		fmt.Fprint(env.Err, "\n")
	}
}

func warnStale(env *registry.Env, meta *index.Object, path string) {
	for _, w := range core.Staleness(meta, path) {
		fmt.Fprintf(env.Err, "WARNING: %s\n", w)
	}
}

// xrefAbsent is _note_absent: true, after the NOTE indexes.load prints, when
// the optional xref index does not exist.
func xrefAbsent(env *registry.Env) bool {
	if st, err := os.Stat(absIndex(env, xrefIndex)); err == nil && st.Mode().IsRegular() {
		return false
	}
	env.Load(xrefIndex, core.LoadOpts{Optional: true})
	return true
}

func load(env *registry.Env, path, name, what string) *index.Object {
	started := progressStart(env, path, name)
	data := env.LoadJSON(path, what)
	progressEnd(env, started, data != nil)
	return data
}

// readInputs is class_rank_cache.read_inputs: (classes, imported_by, meta) of
// the class and xref indexes, or ok=false after an exit-2 error.
func readInputs(env *registry.Env) (classes, importedBy, meta *index.Object, ok bool) {
	path := absIndex(env, classIndex)
	data := load(env, path, classIndex, "class index")
	if data == nil {
		return nil, nil, nil, false
	}
	if v, _ := data.Get("classes"); v != nil {
		classes, _ = v.(*index.Object)
	}
	if classes == nil {
		env.Error(fmt.Sprintf("class index has no 'classes' table: %s", path), "Run: ./reindex.sh")
		return nil, nil, nil, false
	}
	if v, _ := data.Get("_meta"); truthy(v) {
		meta, _ = v.(*index.Object)
	}
	warnStale(env, meta, path)
	if xrefAbsent(env) {
		return classes, nil, meta, true
	}
	xrefPath := absIndex(env, xrefIndex)
	xref := load(env, xrefPath, xrefIndex, "xref index")
	if xref == nil {
		return nil, nil, nil, false
	}
	v, present := xref.Get("class_imported_by")
	if present && v != nil {
		if importedBy, ok = v.(*index.Object); !ok {
			env.Error(fmt.Sprintf("xref index has no 'class_imported_by' table: %s", xrefPath),
				"Run: ./reindex.sh")
			return nil, nil, nil, false
		}
	}
	return classes, importedBy, meta, true
}

// truthy is bool(v) for the `_meta or {}` of a decoded value.
func truthy(v any) bool {
	o, ok := v.(*index.Object)
	return ok && o.Len() > 0
}

// Cmd is cmd_rank: print the classes most relevant to question, best first.
func Cmd(env *registry.Env, question string, limit int) {
	if limit < 1 {
		env.Error(fmt.Sprintf("rank: the limit must be a positive integer, got %d", limit), "")
		return
	}
	r := Load(env)
	if r == nil {
		return
	}
	results := r.rankAll(question)
	terms := strings.Join(r.terms.queryTerms(question), ", ")
	if len(results) == 0 {
		if terms == "" {
			terms = "none"
		}
		env.Miss(fmt.Sprintf("question '%s' (terms: %s)", question, terms), r.scope(),
			"grep '<regex>' for literal text, or search '*Name*' for class names")
		return
	}
	w := env.Out
	fmt.Fprintln(w)
	fmt.Fprintf(w, "  Rank '%s': %s matching classes (terms: %s)\n", question,
		core.Commas(int64(len(results))), terms)
	fmt.Fprintln(w, "  Classes named in the question first, then BM25 x importer prior; "+
		"columns: rank, score, class, module, path")
	fmt.Fprintln(w)
	for k, x := range results[:min(limit, len(results))] {
		fmt.Fprintf(w, "%d\t%s\t%s\t%s\t%s\n", k+1, core.Fixed(x.score, 3), x.name, r.module(x.doc),
			r.st.Paths[x.doc])
	}
	if len(results) > limit {
		fmt.Fprintln(w)
		fmt.Fprintf(w, "  ... and %s more (use -n to show more)\n", core.Commas(int64(len(results)-limit)))
	}
	fmt.Fprintln(w)
}
