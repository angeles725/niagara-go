package core

import (
	"fmt"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/angeles725/niagara-go/internal/difflib"
	"github.com/angeles725/niagara-go/internal/index"
	"github.com/angeles725/niagara-go/internal/textfold"
)

// MaxSuggestions is suggest.MAX_SUGGESTIONS.
const MaxSuggestions = 8

// nameCore is suggest._core: "BConversion" -> "conversion"; other names only
// lose their case.
func nameCore(query string) string {
	r := []rune(query)
	if len(r) > 2 && r[0] == 'B' && unicode.IsUpper(r[1]) {
		return textfold.Lower(string(r[1:]))
	}
	return textfold.Lower(query)
}

// isBForm is `n[:1] == "B" and n[1:2].isupper()`.
func isBForm(n string) bool {
	if !strings.HasPrefix(n, "B") {
		return false
	}
	r, _ := utf8.DecodeRuneInString(n[1:])
	return len(n) > 1 && unicode.IsUpper(r)
}

// CloseNames is suggest.close_names: up to limit names the user probably
// meant, in stages: the name in another case, a submodule of the family the
// query names (alarm -> alarm-rt), names containing the query (or its core
// without the Niagara B prefix) with the same ending and form first, shortest
// first, then difflib's close matches. Each stage keeps its best entries like
// heapq.nsmallest (stable: ties keep the input order).
func CloseNames(query string, names []string, limit int) []string {
	if strings.Contains(query, ".") && !strings.HasSuffix(query, ".jar") {
		query = query[strings.LastIndex(query, ".")+1:]
	}
	if query == "" {
		return nil
	}
	lower, core := textfold.Lower(query), nameCore(query)
	lowered := make([]string, len(names))
	for i, n := range names {
		lowered[i] = textfold.Lower(n)
	}
	var picked []string
	seen := map[string]bool{query: true}
	take := func(ranked []string) {
		for _, n := range ranked {
			if len(picked) >= limit {
				return
			}
			if !seen[n] {
				seen[n] = true
				picked = append(picked, n)
			}
		}
	}
	// add is take(heapq.nsmallest(limit + len(picked), candidates, key=less)).
	add := func(keep func(i int) bool, less func(a, b int) bool) {
		var idx []int
		for i := range names {
			if keep(i) {
				idx = append(idx, i)
			}
		}
		sort.SliceStable(idx, func(x, y int) bool { return less(idx[x], idx[y]) })
		k := max(0, min(len(idx), limit+len(picked))) // nsmallest(n <= 0) is []
		ranked := make([]string, k)
		for j := range k {
			ranked[j] = names[idx[j]]
		}
		take(ranked)
	}
	byName := func(a, b int) bool { return names[a] < names[b] }
	add(func(i int) bool { return lowered[i] == lower }, byName)
	add(func(i int) bool { return strings.HasPrefix(lowered[i], lower+"-") }, byName)
	bForm := strings.HasPrefix(query, "B") && core != lower
	add(func(i int) bool { return strings.Contains(lowered[i], core) }, func(a, b int) bool {
		ea, eb := !strings.HasSuffix(lowered[a], core), !strings.HasSuffix(lowered[b], core)
		if ea != eb {
			return !ea
		}
		fa, fb := isBForm(names[a]) != bForm, isBForm(names[b]) != bForm
		if fa != fb {
			return !fa
		}
		la, lb := utf8.RuneCountInString(names[a]), utf8.RuneCountInString(names[b])
		if la != lb {
			return la < lb
		}
		return names[a] < names[b]
	})
	if len(picked) < limit {
		close, _ := difflib.GetCloseMatches(query, names, limit, 0.75)
		take(close)
	}
	return picked
}

// Report is suggest.report: Miss plus a "Did you mean" line when close names exist.
func (r *Runtime) Report(subject, scope, query string, names []string, hint string) []string {
	r.Miss(subject, scope, hint)
	s := CloseNames(query, names, MaxSuggestions)
	if len(s) > 0 {
		fmt.Fprintf(r.Out, "  Did you mean: %s\n", strings.Join(s, ", "))
	}
	return s
}

// LoadClassIndex is class_search.load_class_index: class-index.json, shared.
func (r *Runtime) LoadClassIndex() *index.Object { return r.Load("class-index.json", LoadOpts{}) }

// ClassScope is class_search.class_scope: the counted description of what a
// class lookup searches.
func ClassScope(data *index.Object) string {
	meta := data.Object("_meta")
	classes := data.Object("classes")
	if classes == nil { // Python raises KeyError; the port counts an empty table
		classes = index.NewObject()
	}
	entries := 0
	mods := map[string]bool{}
	for _, v := range classes.Values() {
		list, _ := v.([]any)
		entries += len(list)
		for _, e := range list {
			if o, ok := e.(*index.Object); ok {
				m, _ := o.Get("module")
				mods[PyStr(m)] = true
			}
		}
	}
	modules := Commas(int64(len(mods)))
	if meta != nil {
		if v, ok := meta.Get("modules"); ok && index.Truthy(v) {
			if n, isInt := index.Int(v); isInt {
				modules = Commas(n)
			} else {
				modules = PyStr(v)
			}
		}
	}
	return fmt.Sprintf("%s class entries (%s names) from %s modules, %s",
		Commas(int64(entries)), Commas(int64(classes.Len())), modules, Built(meta))
}

// ClassMiss is suggest.class_miss: the miss of a class lookup over the class
// index (loaded when data is nil). what is "class" unless the caller says.
func (r *Runtime) ClassMiss(name string, data *index.Object, what, hint string) []string {
	if data == nil {
		if data = r.LoadClassIndex(); data == nil {
			return nil
		}
	}
	if what == "" {
		what = "class"
	}
	var names []string
	if classes := data.Object("classes"); classes != nil {
		names = classes.Keys()
	}
	return r.Report(fmt.Sprintf("%s '%s'", what, name), ClassScope(data), name, names, hint)
}

// ModuleMiss is suggest.module_miss: the miss of a submodule lookup over the
// module inventory (Python's default hint is "modules").
func (r *Runtime) ModuleMiss(name, hint string) []string {
	inv := r.Load("module-inventory.json", LoadOpts{})
	if inv == nil || inv.Len() == 0 {
		return nil
	}
	modules := inv.Object("modules")
	var keys []string
	if modules != nil {
		keys = modules.Keys()
	}
	scope := fmt.Sprintf("%s submodules in module-inventory.json, %s",
		Commas(int64(len(keys))), Built(inv.Object("_meta")))
	return r.Report(fmt.Sprintf("module '%s'", name), scope, name, keys, hint)
}
