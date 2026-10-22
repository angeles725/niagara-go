package corpusnav

import (
	"fmt"
	"math"
	"slices"
	"sort"
	"strings"

	"github.com/angeles725/niagara-go/internal/bm25"
	"github.com/angeles725/niagara-go/internal/corpusnav/blocksearch"
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

type semanticHit struct {
	score float64
	b     blockFile
}

// semanticRanked is _semantic_ranked over blocks counted now (no cache).
func semanticRanked(query string, blocks []blockFile, unreadable *[]string) []semanticHit {
	if len(tfidfTokens(query)) == 0 {
		return nil // read nothing, as _semantic_ranked
	}
	return rankTFIDF(query, blocksearch.CountTerms(blockPaths(blocks), tfidfTokens), blocks, unreadable)
}

func blockPaths(blocks []blockFile) []string {
	out := make([]string, len(blocks))
	for i, b := range blocks {
		out[i] = b.path
	}
	return out
}

// termCounts is the TF-IDF term counts of blocks: through the cache beside the find
// cache when c.termCache (set by Main, so a test pointed at a corpus writes nothing unless
// it asks), else counted now; and the cache state (blocksearch.CachedTermCounts).
func (c *corpus) termCounts(blocks []blockFile) (*blocksearch.TermCounts, string) {
	if !c.termCache {
		return blocksearch.CountTerms(blockPaths(blocks), tfidfTokens), "off"
	}
	targets := make([]blocksearch.Target, len(blocks))
	for i, b := range blocks {
		targets[i] = blocksearch.Target{Display: displayID(b.label), Path: b.path}
	}
	return c.search().CachedTermCounts("tfidf-blocks", targets, tfidfTokens)
}

// rankTFIDF is _semantic_ranked over the term counts of blocks (tc.Docs[i] is blocks[i]):
// cosine similarity of TF-IDF vectors, idf = log(N / (1 + df)) + 1; zero scores dropped;
// the display id of every unreadable block is appended to unreadable. A document's weight
// is count x idf and its norm the L2 norm of its weights in first-occurrence order
// (Python's compensated sum()), computed from the counts without a weight map, and only
// for the documents that share a term with the query (any other one scores 0).
func rankTFIDF(query string, tc *blocksearch.TermCounts, blocks []blockFile, unreadable *[]string) []semanticHit {
	queryTokens := tfidfTokens(query)
	if len(queryTokens) == 0 {
		return nil
	}
	if unreadable != nil {
		for i, d := range tc.Docs {
			if !d.Readable {
				*unreadable = append(*unreadable, displayID(blocks[i].label))
			}
		}
	}
	df := make([]int, len(tc.Vocab))
	for _, d := range tc.Docs {
		for _, id := range d.IDs {
			df[id]++
		}
	}
	idf := make([]float64, len(tc.Vocab))
	for id, n := range df {
		idf[id] = bm25.Log(float64(len(blocks))/float64(1+n)) + 1.0
	}
	// the query vector: its terms in first-occurrence order (a term no block holds weighs 0)
	var qIDs []int
	var qCounts []int
	qPos := map[string]int{}
	for _, t := range queryTokens {
		if k, ok := qPos[t]; ok {
			qCounts[k]++
			continue
		}
		qPos[t] = len(qIDs)
		qIDs = append(qIDs, tc.ID(t))
		qCounts = append(qCounts, 1)
	}
	qvec := make([]float64, len(qIDs))
	squares := make([]float64, len(qIDs))
	inQuery := map[int32]int{}
	for k, id := range qIDs {
		if id >= 0 {
			qvec[k] = float64(qCounts[k]) * idf[id]
			inQuery[int32(id)] = k
		}
		squares[k] = qvec[k] * qvec[k]
	}
	qnorm := math.Sqrt(bm25.PythonSum(squares))
	if qnorm == 0 {
		return nil
	}
	var scored []semanticHit
	docCount := make([]int32, len(qIDs))
	var parts []float64
	for i, d := range tc.Docs {
		clear(docCount)
		shared := false
		for k, id := range d.IDs {
			if q, ok := inQuery[id]; ok {
				docCount[q], shared = d.Counts[k], true
			}
		}
		if !shared {
			continue
		}
		parts = parts[:0]
		for k, id := range qIDs {
			if docCount[k] > 0 {
				parts = append(parts, qvec[k]*(float64(docCount[k])*idf[id]))
			}
		}
		dot := bm25.PythonSum(parts)
		if dot == 0 {
			continue
		}
		squares = slices.Grow(squares[:0], len(d.IDs))
		for k, id := range d.IDs {
			v := float64(d.Counts[k]) * idf[id]
			squares = append(squares, v*v)
		}
		if norm := math.Sqrt(bm25.PythonSum(squares)); norm != 0 {
			scored = append(scored, semanticHit{dot / (qnorm * norm), blocks[i]})
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
	var scored []semanticHit
	if len(tfidfTokens(expanded)) > 0 { // nothing to rank with: read nothing, as _semantic_ranked
		tc, _ := c.termCounts(blocks)
		scored = rankTFIDF(expanded, tc, blocks, &unreadable)
	}
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
	for _, h := range scored[:min(max(limit, 0), len(scored))] {
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
