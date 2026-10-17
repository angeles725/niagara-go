package corpusnav

import (
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/angeles725/niagara-go/internal/bm25"
	"github.com/angeles725/niagara-go/internal/glossary"
	"github.com/angeles725/niagara-go/internal/honest"
	"github.com/angeles725/niagara-go/internal/pycompat"
	"github.com/angeles725/niagara-go/internal/textfold"
)

// corpusnav/semantic.py, TF-IDF only: the fastembed embedding backend is retired in the
// Go port (decision D10), so `semantic` always ranks with the stdlib TF-IDF fallback.

var tfidfStopwords = map[string]bool{}

func init() {
	for _, w := range strings.Fields(`the and for with this that are was were has have had not but from into
		its can will also all any one each when then than they them their there been being out more over such
		how use used using per via get set new add may see two own now well
		que los las del por una con para como sus son sin mas hay ser est`) {
		tfidfStopwords[w] = true
	}
}

// tfidfTokens is _tokenize_for_tfidf: lower-cased, split on non-[a-z0-9], tokens of 3+
// characters that are not stop-words.
func tfidfTokens(text string) []string {
	var out []string
	for _, t := range strings.FieldsFunc(textfold.Lower(text), func(r rune) bool {
		return !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9')
	}) {
		if len(t) >= 3 && !tfidfStopwords[t] {
			out = append(out, t)
		}
	}
	return out
}

// tfidfVector is _tfidf_vector: term weights in first-occurrence order (a Counter) and
// the L2 norm (Python's compensated sum()).
func tfidfVector(tokens []string, idf map[string]float64) ([]string, map[string]float64, float64) {
	var order []string
	counts := map[string]int{}
	for _, t := range tokens {
		if counts[t] == 0 {
			order = append(order, t)
		}
		counts[t]++
	}
	vec := map[string]float64{}
	squares := make([]float64, len(order))
	for i, t := range order {
		vec[t] = float64(counts[t]) * idf[t]
		squares[i] = vec[t] * vec[t]
	}
	return order, vec, math.Sqrt(bm25.PythonSum(squares))
}

type semanticHit struct {
	score float64
	b     blockFile
}

// semanticRanked is _semantic_ranked: cosine similarity of TF-IDF vectors, idf =
// log(N / (1 + df)) + 1; zero scores dropped; the display id of every unreadable block
// is appended to unreadable.
func semanticRanked(query string, blocks []blockFile, unreadable *[]string) []semanticHit {
	queryTokens := tfidfTokens(query)
	if len(queryTokens) == 0 {
		return nil
	}
	docs := make([][]string, len(blocks))
	df := map[string]int{}
	for i, b := range blocks {
		text, ok := readText(b.path)
		if !ok && unreadable != nil {
			*unreadable = append(*unreadable, displayID(b.label))
		}
		docs[i] = tfidfTokens(text)
		seen := map[string]bool{}
		for _, t := range docs[i] {
			if !seen[t] {
				seen[t] = true
				df[t]++
			}
		}
	}
	idf := map[string]float64{}
	for t, n := range df {
		idf[t] = bm25.Log(float64(len(blocks))/float64(1+n)) + 1.0
	}
	qOrder, qvec, qnorm := tfidfVector(queryTokens, idf)
	if qnorm == 0 {
		return nil
	}
	var scored []semanticHit
	for i, b := range blocks {
		_, vec, norm := tfidfVector(docs[i], idf)
		var parts []float64
		for _, t := range qOrder {
			if v, ok := vec[t]; ok {
				parts = append(parts, qvec[t]*v)
			}
		}
		if dot := bm25.PythonSum(parts); norm != 0 && dot != 0 {
			scored = append(scored, semanticHit{dot / (qnorm * norm), b})
		}
	}
	sort.SliceStable(scored, func(i, j int) bool { return scored[i].score > scored[j].score })
	return scored
}

// bestSnippet is _best_snippet: the first line with the most query-term substrings.
func bestSnippet(lines []string, terms map[string]bool) string {
	best, bestCount := "", -1
	for _, line := range lines {
		low, count := textfold.Lower(line), 0
		for t := range terms {
			if strings.Contains(low, t) {
				count++
			}
		}
		if count > bestCount {
			best, bestCount = pycompat.Strip(line), count
		}
	}
	return best
}

// cmdSemantic is cmd_semantic with only its TF-IDF branch.
func (c *corpus) cmdSemantic(query string, hasQuery, buildIndex, forceTFIDF bool, limit int) int {
	if buildIndex {
		fmt.Fprintln(c.err, "corpus-nav: cannot build the embedding index: embeddings are not part of the Go port "+
			"(decision D10); semantic ranks with TF-IDF")
		return 2
	}
	if !hasQuery || query == "" {
		fmt.Fprintln(c.err, "corpus-nav: semantic needs a query (or --build-index)")
		return 2
	}
	catalog := c.loadCatalog()
	blocks := c.blocks()
	g := glossary.Default()
	expanded := g.Expand(query) // a Spanish question also asks for the English terms
	if expanded != query {
		fmt.Fprintf(c.out, "(%s)\n", g.Note(query))
	}
	why := "forced by --tfidf"
	if !forceTFIDF {
		why = "embeddings are not part of the Go port (decision D10)"
	}
	fmt.Fprintf(c.out, "[backend: tfidf (%s)]\n", why)
	terms := map[string]bool{}
	for _, t := range textfold.ParseQuery(expanded) {
		terms[t] = true
	}
	var unreadable []string
	scored := semanticRanked(expanded, blocks, &unreadable)
	c.reportUnreadable(unreadable) // zero false negatives: a block that could not be read is named
	if len(scored) == 0 {
		counts := []honest.Count{{Label: "files", N: len(blocks)}}
		if len(unreadable) > 0 {
			counts = append(counts, honest.Count{Label: "unreadable", N: len(unreadable)})
		}
		fmt.Fprintln(c.out, honest.Miss("semantic matches", "blocks", counts...))
		fmt.Fprintln(c.out, "(Note: absence of semantic matches is not proof of absence.)")
		if len(unreadable) > 0 {
			return 2
		}
		return 1
	}
	for _, h := range scored[:min(limit, len(scored))] {
		fmt.Fprintf(c.out, "%s · %s   (score %.3f)\n", displayID(h.b.label), titleFor(h.b.label, catalog, h.b.path), h.score)
		text, _ := readText(h.b.path)
		if snippet := bestSnippet(pycompat.SplitLines(text, false), terms); snippet != "" {
			if rs := []rune(snippet); len(rs) > 120 {
				snippet = string(rs[:117]) + "..."
			}
			fmt.Fprintf(c.out, "  » %s\n", snippet)
		}
	}
	if len(unreadable) > 0 {
		return 2
	}
	return 0
}
