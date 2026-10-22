package corpusnav

import (
	"math"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/angeles725/niagara-go/internal/bm25"
	"github.com/angeles725/niagara-go/internal/corpusnav/blocksearch"
	"github.com/angeles725/niagara-go/internal/glossary"
)

// referenceRanked is _semantic_ranked as the port first wrote it (a token list and a
// weight map per document): rankTFIDF, from counted or cached terms, must give exactly
// its hits, in its order, with bit-identical scores.
// The documents are tokenized once (referenceDocs) for all the queries.
type referenceDocs struct {
	tokens     [][]string
	unreadable []string
}

func readReferenceDocs(blocks []blockFile) referenceDocs {
	var r referenceDocs
	for _, b := range blocks {
		text, ok := readText(b.path)
		if !ok {
			r.unreadable = append(r.unreadable, displayID(b.label))
		}
		r.tokens = append(r.tokens, tfidfTokens(text))
	}
	return r
}

func referenceRanked(query string, blocks []blockFile, ref referenceDocs) (hits []semanticHit, unreadable []string) {
	vector := func(tokens []string, idf map[string]float64) ([]string, map[string]float64, float64) {
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
	queryTokens := tfidfTokens(query)
	if len(queryTokens) == 0 {
		return nil, nil
	}
	docs, unreadable := ref.tokens, ref.unreadable
	df := map[string]int{}
	for i := range blocks {
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
	qOrder, qvec, qnorm := vector(queryTokens, idf)
	if qnorm == 0 {
		return nil, unreadable
	}
	for i, b := range blocks {
		_, vec, norm := vector(docs[i], idf)
		var parts []float64
		for _, t := range qOrder {
			if v, ok := vec[t]; ok {
				parts = append(parts, qvec[t]*v)
			}
		}
		if dot := bm25.PythonSum(parts); norm != 0 && dot != 0 {
			hits = append(hits, semanticHit{dot / (qnorm * norm), b})
		}
	}
	sort.SliceStable(hits, func(i, j int) bool { return hits[i].score > hits[j].score })
	return hits, unreadable
}

// semanticQueries are the golden queries (glossary-expanded as cmd_semantic does) and
// the corners: stop-words only, no known term, nothing to tokenize.
var semanticQueries = []string{"alarm escalation", "alarm", "escalacion de alarmas", "zzqq",
	"point health fault override status", "alarm webhook", "bacnet bbmd", "the and for", "", "a b",
	"alarm alarm escalation alarm"}

func checkRanking(t *testing.T, name string, blocks []blockFile, tc *blocksearch.TermCounts, queries []string) {
	t.Helper()
	ref := readReferenceDocs(blocks)
	for _, q := range queries {
		q = glossary.Default().Expand(q)
		want, wantUnreadable := referenceRanked(q, blocks, ref)
		var gotUnreadable []string
		got := rankTFIDF(q, tc, blocks, &gotUnreadable)
		if len(got) != len(want) || len(gotUnreadable) != len(wantUnreadable) {
			t.Errorf("%s %q: %d hits %v unreadable, want %d %v", name, q, len(got), gotUnreadable, len(want), wantUnreadable)
			continue
		}
		for i := range want {
			if got[i].b != want[i].b || math.Float64bits(got[i].score) != math.Float64bits(want[i].score) {
				t.Errorf("%s %q hit %d: %s %v, want %s %v", name, q, i, got[i].b.label, got[i].score,
					want[i].b.label, want[i].score)
				break
			}
		}
		for i := range wantUnreadable {
			if gotUnreadable[i] != wantUnreadable[i] {
				t.Errorf("%s %q: unreadable %v, want %v", name, q, gotUnreadable, wantUnreadable)
			}
		}
	}
}

// TestRankTFIDFMatchesReference: on the fixture corpus (plus an unreadable block), the
// ranking from counted terms, from a freshly written cache and from a warm cache.
func TestRankTFIDFMatchesReference(t *testing.T) {
	t.Setenv("CORPUS_NAV_FIND_INDEX", "")
	c, _, _, root := copyCorpus(t)
	c.indexDir, c.termCache = t.TempDir(), true
	if err := os.WriteFile(filepath.Join(root, "niagara-mental-model-bloque9.md"), []byte{0xff, 0xfe}, 0o644); err != nil {
		t.Fatal(err)
	}
	blocks := c.blocks()
	checkRanking(t, "counted", blocks, blocksearch.CountTerms(blockPaths(blocks), tfidfTokens), semanticQueries)
	for _, state := range []string{"missing", "hit"} {
		tc, got := c.termCounts(blocks)
		if got != state {
			t.Errorf("cache state %q, want %q", got, state)
		}
		checkRanking(t, state, blocks, tc, semanticQueries)
	}
}

// TestRankTFIDFMatchesReferenceOnRealCorpus: the same over every main block of the
// research corpus (read-only; the cache goes to a temporary directory). Skipped where
// the corpus is not checked out.
func TestRankTFIDFMatchesReferenceOnRealCorpus(t *testing.T) {
	if testing.Short() {
		t.Skip("reads the whole research corpus")
	}
	t.Setenv("CORPUS_NAV_FIND_INDEX", "")
	root := os.Getenv("CORPUS_NAV_ROOT")
	if root == "" {
		home, _ := os.UserHomeDir()
		root = filepath.Join(home, "niagara-research")
	}
	c := newCorpus(root, t.TempDir(), os.Stdout, os.Stderr)
	c.termCache = true
	blocks := c.blocks()
	if len(blocks) == 0 {
		t.Skipf("no research corpus at %s", root)
	}
	if _, state := c.termCounts(blocks); state != "missing" {
		t.Errorf("first state %q", state)
	}
	tc, state := c.termCounts(blocks)
	if state != "hit" {
		t.Errorf("second state %q", state)
	}
	checkRanking(t, "real", blocks, tc, semanticQueries[4:8]) // the reference takes ~0.5 s a query here
}
