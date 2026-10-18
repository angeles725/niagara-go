package help

// The golden-set half of niagara-help tests/test_search_quality.py
// (TestGoldenSet): hit@1 / hit@5 of the real questions that name an expected
// doc file, ranked by the Go full-text engine on the real 4.14.0.162 root
// corpus named by NIAGARA_HELP_DIR. The questions are a hermetic copy in
// testdata/quality (its header names the source commit). The synthetic
// ranking rules of that Python file live in fts/ranking_test.go.

import (
	"bufio"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/angeles725/niagara-go/internal/help/fts"
	"github.com/angeles725/niagara-go/internal/help/scope"
)

// The Python engine's floors (test_search_quality.py GOLDEN_FLOOR_AT1/AT5,
// T15b). They are the migration criterion: the Go engine must not rank
// fewer questions first or in the top five. Raise them when a ranking change
// lifts more questions; never lower them.
const (
	goldenFloorAt1 = 27
	goldenFloorAt5 = 44
)

// andRecallRows are the questions whose expected manual a small AND set
// (decompiled code or a few devguide pages with every word) used to keep
// out of the results on 4.14 (T15b).
var andRecallRows = []string{"q13", "q64", "q65"}

type goldenQuery struct {
	ID       string `json:"id"`
	Question string `json:"question"`
	Expected struct {
		Doc []string `json:"doc"`
	} `json:"expected"`
}

func loadGoldenQueries(t *testing.T) []goldenQuery {
	t.Helper()
	f, err := os.Open(filepath.Join("testdata", "quality", "golden-doc-queries.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	var rows []goldenQuery
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		var q goldenQuery
		if err := json.Unmarshal([]byte(line), &q); err != nil {
			t.Fatalf("golden query %q: %v", line, err)
		}
		if len(q.Expected.Doc) == 0 {
			t.Fatalf("golden query %s names no expected doc", q.ID)
		}
		rows = append(rows, q)
	}
	if err := sc.Err(); err != nil {
		t.Fatal(err)
	}
	return rows
}

// goldenRank is the 1-based rank of the first hit under one of the expected
// doc paths (a prefix match, as Python's startswith), 0 when none is listed;
// keys are the ranked hit keys.
func goldenRank(keys []string, q goldenQuery) int {
	for i, k := range keys {
		for _, e := range q.Expected.Doc {
			if strings.HasPrefix(k, e) {
				return i + 1
			}
		}
	}
	return 0
}

// realSearch ranks question on the real corpus as `find` does (every source,
// AND with its fallback), building the index first when it is missing or
// stale like nh does.
func realSearch(t *testing.T, base, question string) []string {
	t.Helper()
	r, err := fts.SearchUsing(scope.OpenFulltext(io.Discard), base, realVersion, question, nil, "and")
	if err != nil {
		t.Fatalf("search %q: %v", question, err)
	}
	keys := make([]string, len(r.Hits))
	for i, h := range r.Hits {
		keys[i] = h.Key()
	}
	return keys
}

// TestQualityRealGoldenDocHits: the golden doc set must not regress below
// the Python engine's floors.
func TestQualityRealGoldenDocHits(t *testing.T) {
	_, base := realCorpus(t)
	rows := loadGoldenQueries(t)
	at1, at5 := 0, 0
	var below []string
	for _, q := range rows {
		rank := goldenRank(realSearch(t, base, q.Question), q)
		if rank == 1 {
			at1++
		}
		if rank >= 1 && rank <= 5 {
			at5++
		}
		if rank != 1 {
			below = append(below, q.ID+" rank "+rankText(rank)+": "+q.Question)
		}
	}
	t.Logf("golden doc set: hit@1 %d/%d  hit@5 %d/%d (floors %d, %d)", at1, len(rows), at5, len(rows),
		goldenFloorAt1, goldenFloorAt5)
	if at1 < goldenFloorAt1 || at5 < goldenFloorAt5 {
		t.Errorf("golden doc set regressed: hit@1 %d (floor %d), hit@5 %d (floor %d); not first:\n  %s",
			at1, goldenFloorAt1, at5, goldenFloorAt5, strings.Join(below, "\n  "))
	}
}

func rankText(rank int) string {
	if rank == 0 {
		return "none"
	}
	return strconv.Itoa(rank)
}

// TestQualityRealManualsNotHiddenBySmallAndSet: on 4.14 these questions had
// a small AND set that kept the expected manual, which matches most words,
// out of the results (T15b); it must be in the top five.
func TestQualityRealManualsNotHiddenBySmallAndSet(t *testing.T) {
	_, base := realCorpus(t)
	byID := map[string]goldenQuery{}
	for _, q := range loadGoldenQueries(t) {
		byID[q.ID] = q
	}
	for _, id := range andRecallRows {
		q, ok := byID[id]
		if !ok {
			t.Fatalf("golden query %s is missing from testdata/quality", id)
		}
		keys := realSearch(t, base, q.Question)
		if rank := goldenRank(keys, q); rank < 1 || rank > 5 {
			t.Errorf("%s: expected %q in the top 5, got %q", id, q.Expected.Doc, keys[:min(5, len(keys))])
		}
	}
}
