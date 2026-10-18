// Package callgraph is the callers/callees lookup of module-navigator
// (callgraph.py) over indexes/callgraph-index.json, as a library (decision
// D29): it prints nothing, returns typed rows and exports the formatters that
// produce the CLI lines, so module-nav and nsearch show the same text.
//
// Python builds a reverse index ("Class.method" -> sorted unique callers,
// each callee filed under every dotted suffix of two or more segments) and
// looks keys up in it. The port answers the same questions by scanning the
// call edges once per lookup instead of materializing that index (about a
// million keys): the keys a callee is filed under are exactly its suffixes
// that start after a '.' and still hold a '.', plus the whole name, and
// "first key in the reverse index" is "first suffix met while scanning the
// callers and their callees in index order".
package callgraph

import (
	"fmt"
	"slices"
	"strings"

	"github.com/angeles725/niagara-go/internal/index"
	"github.com/angeles725/niagara-go/internal/nav/core"
	"github.com/angeles725/niagara-go/internal/textfold"
)

// Graph is a loaded call graph: every caller with its callees, in index order.
type Graph struct {
	meta    *index.Object
	callers []string
	callees [][]string
}

// New wraps the decoded callgraph-index.json (its "calls" and "_meta").
func New(cg *index.Object) *Graph {
	g := &Graph{meta: cg.Object("_meta")}
	if calls := cg.Object("calls"); calls != nil {
		g.callers = calls.Keys()
		g.callees = make([][]string, len(g.callers))
		for i, v := range calls.Values() {
			list, _ := v.([]any)
			out := make([]string, len(list))
			for j, c := range list {
				out[j] = core.PyStr(c)
			}
			g.callees[i] = out
		}
	}
	return g
}

// Scope is _edges_scope: what a callers lookup searched, for a miss line.
func (g *Graph) Scope() string {
	edges := int64(0)
	if g.meta != nil {
		if v, ok := g.meta.Get("total_edges"); ok {
			edges, _ = index.Int(v)
		}
	}
	return fmt.Sprintf("%s call edges from %s caller methods (static, this/super, new and "+
		"Class.method calls; calls on instance variables are not resolved), %s",
		core.Commas(edges), core.Commas(int64(len(g.callers))), core.Built(g.meta))
}

// keyStarts calls f with the start of every reverse-index key of callee, in
// Python's order: the whole name, then each suffix after a '.' that still
// holds a '.'.
func keyStarts(callee string, f func(start int) bool) {
	if f(0) {
		return
	}
	last := strings.LastIndexByte(callee, '.')
	for i := 0; i < last; i++ {
		if callee[i] == '.' && f(i+1) {
			return
		}
	}
}

// fileUnder reports whether callee is filed under key.
func fileUnder(callee, key string) bool {
	if callee == key {
		return true
	}
	if !strings.HasSuffix(callee, key) || !strings.Contains(key, ".") {
		return false
	}
	return len(callee) > len(key) && callee[len(callee)-len(key)-1] == '.'
}

// CalledBy is called_by[key]: the sorted unique callers filed under key.
func (g *Graph) CalledBy(key string) []string {
	var out []string
	for i, list := range g.callees {
		for _, c := range list {
			if fileUnder(c, key) {
				out = append(out, g.callers[i])
				break
			}
		}
	}
	slices.Sort(out)
	return slices.Compact(out)
}

// AnyCase is outcome.any_case(called_by, key) for a key that is not in the
// reverse index: the first key equal to it ignoring case, else key.
func (g *Graph) AnyCase(key string) string {
	lower := textfold.Lower(key)
	for _, list := range g.callees {
		for _, c := range list {
			found := ""
			keyStarts(c, func(s int) bool {
				if sameLower(c[s:], key, lower) {
					found = c[s:]
				}
				return found != ""
			})
			if found != "" {
				return found
			}
		}
	}
	return key
}

// sameLower is a.lower() == b.lower(), with bLower = b.lower().
func sameLower(a, b, bLower string) bool {
	if isASCII(a) && isASCII(b) {
		return len(a) == len(b) && strings.EqualFold(a, b)
	}
	return textfold.Lower(a) == bLower
}

func isASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] >= 0x80 {
			return false
		}
	}
	return true
}

// classPart is key.rpartition(".")[0].
func classPart(key string) string {
	if i := strings.LastIndexByte(key, '.'); i >= 0 {
		return key[:i]
	}
	return ""
}

// methodPart is key.rpartition(".")[2].
func methodPart(key string) string { return key[strings.LastIndexByte(key, '.')+1:] }

// counted is one reverse-index key with its callers.
type counted struct {
	key     string
	callers []string
}

// collect groups the (key, caller) pairs of the reverse index whose key
// satisfies match, keys in index order, callers sorted and unique.
func (g *Graph) collect(match func(key string) bool) []counted {
	pos := map[string]int{}
	var out []counted
	for i, list := range g.callees {
		for _, c := range list {
			keyStarts(c, func(s int) bool {
				k := c[s:]
				if !match(k) {
					return false
				}
				j, ok := pos[k]
				if !ok {
					j = len(out)
					pos[k] = j
					out = append(out, counted{key: k})
				}
				if n := len(out[j].callers); n == 0 || out[j].callers[n-1] != g.callers[i] {
					out[j].callers = append(out[j].callers, g.callers[i])
				}
				return false
			})
		}
	}
	for j := range out {
		slices.Sort(out[j].callers)
		out[j].callers = slices.Compact(out[j].callers)
	}
	return out
}

// ModuleOf is _get_module_for_class: the module of the first top-level entry
// of class in the class index table (else of its first entry), "?" when unknown.
func ModuleOf(classes *index.Object, class string) string {
	if classes == nil {
		return "?"
	}
	v, ok := classes.Get(class)
	if !ok {
		return "?"
	}
	entries := core.Objects(v)
	for _, e := range entries {
		if !core.Truthy(e, "outer_class") {
			return moduleField(e)
		}
	}
	if len(entries) > 0 {
		return moduleField(entries[0])
	}
	return "?"
}

func moduleField(e *index.Object) string {
	if m, ok := e.Get("module"); ok {
		return core.PyStr(m)
	}
	return "?"
}
