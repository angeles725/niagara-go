package classrank

import (
	"fmt"
	"sort"
	"strings"

	"github.com/angeles725/niagara-go/internal/bm25"
	"github.com/angeles725/niagara-go/internal/index"
	"github.com/angeles725/niagara-go/internal/nav/core"
)

// noModule is the module index of a class entry whose module is null.
const noModule = -1

// state is the prepared ranker (ClassRanker.state()): plain data, one row per
// document, that the cache stores as is. Bodies and modules repeat across
// classes, so documents point into shared tables.
type state struct {
	Names     []string   // per document: class name
	Mods      []int32    // per document: index into ModTable, noModule for null
	Paths     []string   // per document: str(entry path) ("None" for null)
	BodyIDs   []int32    // per document: index into Bodies
	Haystacks []string   // per document: squash(name + " " + body), the prefilter
	Lengths   []int32    // per document: titleWeight x name parts + body parts
	Priors    []float64  // per document: 1 + 0.3 x log1p(distinct importers of its name)
	ModTable  []string   // distinct modules, first-seen order
	Bodies    [][]string // distinct (package, module, superclass, *interfaces), non-empty strings
}

// Ranker is ClassRanker: BM25 over the top-level classes of a class index,
// times an importer prior.
type Ranker struct {
	st     state
	meta   *index.Object
	names  map[string]bool
	avgLen float64
	terms  *terms
}

// isResult is is_result: false for the docSource-doc mirror and inner classes.
func isResult(e *index.Object) bool {
	m, _ := e.Get("module")
	if s, ok := m.(string); ok && s == docMirror {
		return false
	}
	return !core.Truthy(e, "outer_class")
}

// body is filter(None, (package, module, extends, *implements)) of an entry.
func body(e *index.Object) []string {
	var out []string
	add := func(v any) {
		if s, ok := v.(string); ok && s != "" {
			out = append(out, s)
		}
	}
	for _, k := range []string{"package", "module", "extends"} {
		v, _ := e.Get(k)
		add(v)
	}
	for _, v := range core.List(e, "implements") {
		add(v)
	}
	return out
}

// build prepares the state of classes (class-index.json `classes`, in its key
// order) and importedBy (xref class_imported_by, may be nil).
func build(classes, importedBy *index.Object) state {
	var st state
	modIdx, bodyIdx := map[string]int32{}, map[string]int32{}
	for i, name := range classes.Keys() {
		for _, e := range core.Objects(classes.Values()[i]) {
			if !isResult(e) {
				continue
			}
			mod := int32(noModule)
			if m, ok := e.Get("module"); ok && m != nil {
				s := core.PyStr(m)
				id, seen := modIdx[s]
				if !seen {
					id = int32(len(st.ModTable))
					modIdx[s] = id
					st.ModTable = append(st.ModTable, s)
				}
				mod = id
			}
			b := body(e)
			key := strings.Join(b, "\x00")
			bid, seen := bodyIdx[key]
			if !seen {
				bid = int32(len(st.Bodies))
				bodyIdx[key] = bid
				st.Bodies = append(st.Bodies, b)
			}
			length := titleWeight * len(parts(name))
			for _, f := range b {
				length += partCount(f)
			}
			st.Names = append(st.Names, name)
			st.Mods = append(st.Mods, mod)
			st.Paths = append(st.Paths, core.Str(e, "path"))
			st.BodyIDs = append(st.BodyIDs, bid)
			st.Haystacks = append(st.Haystacks, squash(name+" "+strings.Join(b, " ")))
			st.Lengths = append(st.Lengths, int32(length))
		}
	}
	prior := map[string]float64{}
	st.Priors = make([]float64, len(st.Names))
	for i, name := range st.Names {
		p, ok := prior[name]
		if !ok {
			p = 1 + float64(priorWeight*bm25.Log1p(float64(distinct(importedBy, name))))
			prior[name] = p
		}
		st.Priors[i] = p
	}
	return st
}

// distinct is len(set(imported_by.get(name, ()))).
func distinct(importedBy *index.Object, name string) int {
	if importedBy == nil {
		return 0
	}
	v, _ := importedBy.Get(name)
	seen := map[string]bool{}
	for _, x := range func() []any { l, _ := v.([]any); return l }() {
		seen[core.PyRepr(x)] = true
	}
	return len(seen)
}

// newRanker is ClassRanker.from_state: the ranker of a prepared state and the
// class index _meta (nil when absent).
func newRanker(st state, meta *index.Object) (*Ranker, error) {
	n := len(st.Names)
	if len(st.Mods) != n || len(st.Paths) != n || len(st.BodyIDs) != n || len(st.Haystacks) != n ||
		len(st.Lengths) != n || len(st.Priors) != n {
		return nil, fmt.Errorf("malformed ranker state")
	}
	r := &Ranker{st: st, meta: meta, names: map[string]bool{}, terms: newTerms()}
	total := 0
	for i := 0; i < n; i++ {
		if st.Mods[i] < noModule || int(st.Mods[i]) >= len(st.ModTable) ||
			st.BodyIDs[i] < 0 || int(st.BodyIDs[i]) >= len(st.Bodies) {
			return nil, fmt.Errorf("malformed ranker state")
		}
		r.names[st.Names[i]] = true
		total += int(st.Lengths[i])
	}
	if n > 0 {
		r.avgLen = float64(total) / float64(n)
	}
	return r, nil
}

// module is the module of document i as the output prints it (str(None) for null).
func (r *Ranker) module(i int) string {
	if r.st.Mods[i] == noModule {
		return "None"
	}
	return r.st.ModTable[r.st.Mods[i]]
}

// sortModule is `entry.get("module") or ""`.
func (r *Ranker) sortModule(i int) string {
	if r.st.Mods[i] == noModule {
		return ""
	}
	return r.st.ModTable[r.st.Mods[i]]
}

// tf is _tf: the weighted frequency of term in document i.
func (r *Ranker) tf(i int, term string) int {
	n := 0
	for _, t := range r.terms.identifier(r.st.Names[i]) {
		if t == term {
			n = titleWeight
			break
		}
	}
	for _, f := range r.st.Bodies[r.st.BodyIDs[i]] {
		n += r.terms.fieldCounts(f)[term]
	}
	return n
}

// exactNames is exact_names: class names written in the question, in question
// order; "HistoryService" also names BHistoryService (the Baja "B" prefix).
func (r *Ranker) exactNames(question string) []string {
	seen := map[string]bool{}
	var out []string
	for _, tok := range tokens(fold(question)) {
		if len(tok) < 4 || !isIdentifier(tok) {
			continue
		}
		for _, name := range []string{tok, "B" + tok} {
			if r.names[name] {
				if !seen[name] {
					seen[name] = true
					out = append(out, name)
				}
				break
			}
		}
	}
	return out
}

// result is one ranked class: its best-scoring document.
type result struct {
	name  string
	doc   int
	score float64
}

// rankAll is rank_all: every class sharing a term with the question, one per
// class name, named classes first (in question order), then by score, name and
// module. Scores add term contributions in query-term order; documents are met
// in the order Python's dict meets them, which decides ties within one name.
func (r *Ranker) rankAll(question string) []result {
	p := bm25.ClassRank
	type posting struct {
		doc, tf int
	}
	var order []int // documents in first-scored order
	scores := map[int]float64{}
	for _, t := range r.terms.queryTerms(question) {
		var post []posting
		for i, h := range r.st.Haystacks {
			if strings.Contains(h, t) {
				if n := r.tf(i, t); n > 0 {
					post = append(post, posting{i, n})
				}
			}
		}
		if len(post) == 0 {
			continue
		}
		w := p.Weight(len(r.st.Names), len(post))
		for _, x := range post {
			norm := p.Norm(float64(r.st.Lengths[x.doc]), r.avgLen)
			s, seen := scores[x.doc]
			if !seen {
				order = append(order, x.doc)
			}
			scores[x.doc] = s + p.Term(w, float64(x.tf), norm)
		}
	}
	exact := map[string]int{}
	for k, name := range r.exactNames(question) {
		exact[name] = k
	}
	best := map[string]int{} // name -> index into out
	var out []result
	for _, i := range order {
		name := r.st.Names[i]
		s := scores[i] * r.st.Priors[i]
		if k, ok := best[name]; !ok {
			best[name] = len(out)
			out = append(out, result{name, i, s})
		} else if s > out[k].score {
			out[k] = result{name, i, s}
		}
	}
	rankOf := func(name string) int {
		if k, ok := exact[name]; ok {
			return k
		}
		return len(exact)
	}
	sort.SliceStable(out, func(a, b int) bool {
		x, y := out[a], out[b]
		if ra, rb := rankOf(x.name), rankOf(y.name); ra != rb {
			return ra < rb
		}
		if x.score != y.score {
			return x.score > y.score
		}
		if x.name != y.name {
			return x.name < y.name
		}
		return r.sortModule(x.doc) < r.sortModule(y.doc)
	})
	return out
}

// scope is ClassRanker.scope: the counted description of what rank searches.
func (r *Ranker) scope() string {
	mods := map[int32]bool{}
	for _, m := range r.st.Mods {
		mods[m] = true
	}
	return fmt.Sprintf("%s ranked classes (top-level, %s mirror excluded) from %s modules, %s",
		core.Commas(int64(len(r.names))), docMirror, core.Commas(int64(len(mods))), core.Built(r.meta))
}
