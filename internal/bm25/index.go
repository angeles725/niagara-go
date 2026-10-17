package bm25

import "github.com/angeles725/niagara-go/internal/textfold"

// Doc is one document of an Index: its key, title and body.
type Doc[K any] struct {
	Key         K
	Title, Body string
}

// Hit is one ranked document of an Index.
type Hit[K any] struct {
	Score float64
	Key   K
}

// Index is textsearch.BM25: an in-memory index over the textfold ranking
// terms (textfold.Terms) of each document, title terms counted titleWeight times.
type Index[K any] struct {
	p     Params
	keys  []K
	tfs   []map[string]int
	stats Stats
}

// NewIndex builds the index; textsearch.BM25 uses Textsearch and titleWeight 8.
func NewIndex[K any](docs []Doc[K], p Params, titleWeight int) *Index[K] {
	ix := &Index[K]{p: p, stats: Stats{DF: map[string]int{}}}
	total := 0
	for _, d := range docs {
		tf := map[string]int{}
		for _, part := range []struct {
			weight int
			text   string
		}{{1, d.Body}, {titleWeight, d.Title}} {
			for _, tok := range textfold.Tokens(textfold.Fold(part.text, textfold.Corpus)) {
				if t := textfold.Normalize(tok); t != "" {
					tf[t] += part.weight
				}
			}
		}
		n := 0
		for t, c := range tf {
			n += c
			ix.stats.DF[t]++
		}
		ix.keys = append(ix.keys, d.Key)
		ix.tfs = append(ix.tfs, tf)
		ix.stats.Len = append(ix.stats.Len, n)
		total += n
	}
	ix.stats.NDocs = len(docs)
	if len(docs) > 0 {
		ix.stats.AvgLen = float64(total) / float64(len(docs))
	}
	return ix
}

// Rank is textsearch.BM25.rank: the query's terms (textfold.QueryTerms) plus
// the factored terms not among them, ranked by Params.Rank.
func (ix *Index[K]) Rank(query string, requireAll bool, factors []Factor) []Hit[K] {
	q := textfold.QueryTerms(query)
	for _, f := range factors {
		if !contains(q, f.Term) {
			q = append(q, f.Term)
		}
	}
	st := ix.stats
	st.Post = map[string]map[int]int{}
	for _, t := range q {
		if st.DF[t] == 0 {
			continue
		}
		post := map[int]int{}
		for i, tf := range ix.tfs {
			if n, ok := tf[t]; ok {
				post[i] = n
			}
		}
		st.Post[t] = post
	}
	ranked := ix.p.Rank(q, st, requireAll, factors)
	out := make([]Hit[K], len(ranked))
	for i, r := range ranked {
		out[i] = Hit[K]{r.Score, ix.keys[r.Doc]}
	}
	return out
}

func contains(xs []string, x string) bool {
	for _, y := range xs {
		if y == x {
			return true
		}
	}
	return false
}
