// Package bm25 holds the BM25 scoring of the niagara-research tools, with the
// three parameter sets the Python tools use (docs/text-variants.md) and the
// glibc logarithms that keep every score bit-identical to CPython's.
package bm25

import (
	"math"
	"sort"
)

// IDF selects the inverse-document-frequency formula.
type IDF int

const (
	// IDFPlusOne is ln(1 + (N - n + 0.5) / (n + 0.5)), always > 0
	// (textsearch.idf, module-navigator class_rank).
	IDFPlusOne IDF = iota
	// IDFFloored is ln((N - n + 0.5) / (n + 0.5)), replaced by 1e-6 when it is
	// not positive (SQLite FTS5 bm25(), niagara-help fulltext_scan).
	IDFFloored
)

// Sum selects how a document's per-term contributions are added.
type Sum int

const (
	// SumPython is Python's built-in sum() of floats (3.12+): Neumaier
	// compensated summation (textsearch.bm25_rank).
	SumPython Sum = iota
	// SumSequential is plain left-to-right += (class_rank, fulltext_scan).
	SumSequential
)

// Params is one BM25 variant.
type Params struct {
	K1, B float64
	IDF   IDF
	Sum   Sum
}

// The variants of the Python tools.
var (
	// Textsearch is tools/lib/textsearch.py (BM25 defaults and bm25_rank).
	Textsearch = Params{K1: 1.5, B: 0.5, IDF: IDFPlusOne, Sum: SumPython}
	// ClassRank is module-navigator class_rank.py (K1, B).
	ClassRank = Params{K1: 1.5, B: 0.3, IDF: IDFPlusOne, Sum: SumSequential}
	// FTS5 is niagara-help fulltext_scan.py, the FTS5 bm25() defaults.
	FTS5 = Params{K1: 1.2, B: 0.75, IDF: IDFFloored, Sum: SumSequential}
)

// Weight is the idf of a term found in df of nDocs documents.
func (p Params) Weight(nDocs, df int) float64 {
	ratio := (float64(nDocs-df) + 0.5) / (float64(df) + 0.5)
	if p.IDF == IDFFloored {
		if w := Log(ratio); w > 0 {
			return w
		}
		return 1e-6
	}
	return Log(1 + ratio)
}

// Norm is k1 * (1 - b + b * docLen / avgLen), the length normalization of one
// document (Python evaluates it as k1 * ((1 - b) + (b * docLen) / avgLen)).
func (p Params) Norm(docLen, avgLen float64) float64 {
	return float64(p.K1 * ((1 - p.B) + float64(p.B*docLen)/avgLen))
}

// Term is one term's contribution w * tf * (k1 + 1) / (tf + norm).
func (p Params) Term(w, tf, norm float64) float64 {
	return float64(float64(w*tf)*(p.K1+1)) / (tf + norm)
}

// Add sums contributions the way p.Sum says.
func (p Params) Add(xs []float64) float64 {
	if p.Sum == SumPython {
		return PythonSum(xs)
	}
	s := 0.0
	for _, x := range xs {
		s += x
	}
	return s
}

// PythonSum is CPython's sum() over floats: Neumaier summation, the
// compensation added at the end when it is non-zero and finite.
func PythonSum(xs []float64) float64 {
	if len(xs) == 0 {
		return 0
	}
	f, c := 0+xs[0], 0.0
	for _, x := range xs[1:] {
		t := f + x
		if math.Abs(f) >= math.Abs(x) {
			c += (f - t) + x
		} else {
			c += (x - t) + f
		}
		f = t
	}
	if c != 0 && !math.IsInf(c, 0) && !math.IsNaN(c) {
		f += c
	}
	return f
}

// Factor multiplies one term's idf (the glossary's English translations come
// in at 0.5). Order matters: factored terms join the query in this order.
type Factor struct {
	Term   string
	Weight float64
}

// Stats is the collection a query is ranked against.
type Stats struct {
	NDocs  int
	AvgLen float64
	DF     map[string]int         // term -> document frequency (absent = 0)
	Post   map[string]map[int]int // term -> doc -> tf, for every query term that occurs
	Len    []int                  // doc -> document length
}

// Scored is one ranked document.
type Scored struct {
	Score float64
	Doc   int
}

// Rank is textsearch.bm25_rank: the documents sharing a weighted query term,
// best first, equal scores by doc ascending. q is the distinct query terms in
// order; a term without documents carries no weight. requireAll keeps only
// documents holding every term of q. factors multiply the idf of their term.
func (p Params) Rank(q []string, st Stats, requireAll bool, factors []Factor) []Scored {
	factor := map[string]float64{}
	for _, f := range factors {
		factor[f.Term] = f.Weight
	}
	type weighted struct {
		term string
		w    float64
	}
	var ws []weighted
	for _, t := range q {
		if st.DF[t] > 0 {
			w := p.Weight(st.NDocs, st.DF[t])
			if f, ok := factor[t]; ok {
				w *= f
			}
			ws = append(ws, weighted{t, w})
		}
	}
	docs := map[int]bool{}
	for _, x := range ws {
		for d := range st.Post[x.term] {
			docs[d] = true
		}
	}
	var out []Scored
	parts := make([]float64, 0, len(ws))
	for d := range docs {
		if requireAll && !holdsAll(st.Post, q, d) {
			continue
		}
		norm := p.Norm(float64(st.Len[d]), st.AvgLen)
		parts = parts[:0]
		for _, x := range ws {
			if tf, ok := st.Post[x.term][d]; ok {
				parts = append(parts, p.Term(x.w, float64(tf), norm))
			}
		}
		if s := p.Add(parts); s > 0 {
			out = append(out, Scored{s, d})
		}
	}
	sortScored(out)
	return out
}

func holdsAll(post map[string]map[int]int, q []string, d int) bool {
	for _, t := range q {
		if _, ok := post[t][d]; !ok {
			return false
		}
	}
	return true
}

// sortScored orders by score descending, then doc ascending.
func sortScored(xs []Scored) {
	sort.Slice(xs, func(i, j int) bool {
		if xs[i].Score != xs[j].Score {
			return xs[i].Score > xs[j].Score
		}
		return xs[i].Doc < xs[j].Doc
	})
}
