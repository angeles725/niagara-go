package searchdoor

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"
)

// The search-quality gate of niagara-research tools/searchdoor/golden.py, run in process on
// the real corpus: hit@1 / hit@5 per source over the golden question set, the docs pinned to
// the version the set was authored against, plus the warm latency per query.
//
// Opt-in (it needs a niagara-research checkout with its corpora): set QualityRootEnv to the
// checkout, as `make parity` does. The NSEARCH_* variables re-point the backends as for nsearch.

// QualityRootEnv names the niagara-research checkout the quality test runs against.
const QualityRootEnv = "NSEARCH_QUALITY_ROOT"

const (
	qualityFixture    = "testdata/quality/golden-queries.jsonl"
	qualityDocVersion = "4.14.0.162" // golden.py DOC_VERSION
	qualityLimit      = 5            // golden.py --limit default
	qualityTimeout    = 60           // door.search default
	hitDepth          = 5            // a ranked list hits within its first five rows
)

// qualitySet is a range of fixture rows (qNN, inclusive) with its own floor per source:
// hit@1, hit@5 and the questions that expect the source. A run below either hit count fails,
// a run above it is reported so the floor can be raised. Each set keeps the floor it was
// measured with, so rows added later cannot hide a regression of the earlier ones.
type qualitySet struct {
	name     string
	from, to int
	floor    map[string][3]int
}

var qualitySets = []qualitySet{
	{"q01-q81", 1, 81, map[string][3]int{"blocks": {62, 77, 81}, "docs": {36, 62, 67}, "code": {30, 53, 62}}},
	// D41: course questions from the labs.md *Why ...?* items (niagara-research
	// tools/tests/fixtures/golden-queries-review.md, "D41 course rows")
	{"q82-q99 course", 82, 99, map[string][3]int{"blocks": {10, 15, 18}, "docs": {0, 1, 2}, "code": {10, 13, 16}}},
}

// setOf is the index of the quality set holding row id (qNN), or -1.
func setOf(id string) int {
	n, err := strconv.Atoi(strings.TrimPrefix(id, "q"))
	if err != nil {
		return -1
	}
	for i, s := range qualitySets {
		if s.from <= n && n <= s.to {
			return i
		}
	}
	return -1
}

// qualitySources are golden.py SOURCES: the outcome key and the fixture's expectation key.
var qualitySources = [][2]string{{"blocks", "blocks"}, {"docs", "doc"}, {"code", "code"}}

type qualityRow struct {
	ID       string `json:"id"`
	Question string `json:"question"`
	Lang     string `json:"lang"`
	Expected struct {
		Blocks []int    `json:"blocks"`
		Doc    []string `json:"doc"`
		Code   []string `json:"code"`
	} `json:"expected"`
}

func loadQualityRows(t *testing.T) []qualityRow {
	t.Helper()
	raw, err := os.ReadFile(qualityFixture)
	if err != nil {
		t.Fatal(err)
	}
	var rows []qualityRow
	sc := bufio.NewScanner(bytes.NewReader(raw))
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	for sc.Scan() {
		if strings.TrimSpace(sc.Text()) == "" {
			continue
		}
		var r qualityRow
		if err := json.Unmarshal(sc.Bytes(), &r); err != nil {
			t.Fatalf("%s: %v", qualityFixture, err)
		}
		rows = append(rows, r)
	}
	if err := sc.Err(); err != nil {
		t.Fatal(err)
	}
	return rows
}

// wanted is golden.wanted: blocks by label, docs by path prefix, code by the file's stem.
func (r qualityRow) wanted(source string) []string {
	var out []string
	switch source {
	case "blocks":
		for _, b := range r.Expected.Blocks {
			out = append(out, fmt.Sprintf("B%d", b))
		}
	case "docs":
		out = r.Expected.Doc
	case "code":
		for _, p := range r.Expected.Code {
			base := filepath.Base(filepath.FromSlash(p))
			out = append(out, strings.TrimSuffix(base, filepath.Ext(base)))
		}
	}
	return out
}

// hitRank is golden.hit_rank: the 1-based rank of the first row answering want, or 0.
// Docs match by path prefix, blocks and code by exact key; unkeyed rows never match.
func hitRank(source string, want []string, items []Item) int {
	for i, it := range items {
		if !it.Keyed {
			continue
		}
		for _, w := range want {
			if (source == "docs" && strings.HasPrefix(it.Key, w)) || (source != "docs" && it.Key == w) {
				return i + 1
			}
		}
	}
	return 0
}

// sourceRank is golden.source_rank: the source's rows and each sub-scope are separate lists;
// the best rank wins; an enumeration hits anywhere in the rows it shows.
func sourceRank(source string, want []string, o *Outcome) (best int, hit bool) {
	for _, part := range append([]*Outcome{o}, o.Subs...) {
		rank := hitRank(source, want, part.Items)
		if rank == 0 {
			continue
		}
		depth := hitDepth
		if part.Enumeration {
			depth = part.Limit
		}
		hit = hit || rank <= depth
		if best == 0 || rank < best {
			best = rank
		}
	}
	return best, hit
}

// percentile is golden.percentile (Python round: half to even).
func percentile(values []time.Duration, q float64) time.Duration {
	s := append([]time.Duration(nil), values...)
	sort.Slice(s, func(i, j int) bool { return s[i] < s[j] })
	i := int(math.RoundToEven(q * float64(len(s)-1)))
	return s[min(len(s)-1, max(0, i))]
}

func TestSearchQuality(t *testing.T) {
	root := os.Getenv(QualityRootEnv)
	if root == "" {
		t.Skip("set " + QualityRootEnv + " to a niagara-research checkout (make parity)")
	}
	if testing.Short() {
		t.Skip("the real-corpus search gate is not a short test (make parity)")
	}
	t.Setenv("NSEARCH_ROOT", root)
	rows := loadQualityRows(t)
	paths := defaultPaths(os.Getenv)
	d := newDoor()
	d.bind(os.Getenv)
	search := func(q string) map[string]*Outcome {
		out := map[string]*Outcome{}
		for _, o := range d.search(q, paths, qualityLimit, "", qualityTimeout, qualityDocVersion) {
			out[o.Key] = o
		}
		return out
	}
	search(rows[0].Question) // warm-up (caches, page cache)

	type tally struct{ h1, h5, n int }
	hits := map[string]*tally{}
	byLang := map[string]map[string]*tally{}
	bySet := make([]map[string]*tally, len(qualitySets))
	for i := range bySet {
		bySet[i] = map[string]*tally{}
	}
	misses := map[string][]string{}
	unavail := map[string]int{}
	answered := 0
	var times []time.Duration
	for _, row := range rows {
		set := setOf(row.ID)
		if set < 0 {
			t.Fatalf("%s: the row is in no quality set", row.ID)
		}
		t0 := time.Now()
		outcomes := search(row.Question)
		times = append(times, time.Since(t0))
		anyHit := false
		for _, src := range qualitySources {
			source := src[0]
			o := outcomes[source]
			if o == nil {
				t.Fatalf("%s: no %s outcome", row.ID, source)
			}
			if o.State == Unavailable {
				unavail[source]++
			}
			want := row.wanted(source)
			if len(want) == 0 {
				continue
			}
			rank, hit := sourceRank(source, want, o)
			if byLang[row.Lang] == nil {
				byLang[row.Lang] = map[string]*tally{}
			}
			for _, m := range []map[string]*tally{hits, byLang[row.Lang], bySet[set]} {
				if m[source] == nil {
					m[source] = &tally{}
				}
				if rank == 1 {
					m[source].h1++
				}
				if hit {
					m[source].h5++
				}
				m[source].n++
			}
			anyHit = anyHit || hit
			if !hit {
				miss := row.ID
				if rank > 0 {
					miss += fmt.Sprintf("@%d", rank)
				}
				misses[source] = append(misses[source], miss)
			}
		}
		if anyHit {
			answered++
		}
	}

	pooled, total := 0, 0
	for _, src := range qualitySources {
		source := src[0]
		h := hits[source]
		pooled, total = pooled+h.h5, total+h.n
		t.Logf("%-7s hit@1 %d/%d hit@5 %d/%d   unavailable runs: %d   misses: %s",
			source, h.h1, h.n, h.h5, h.n, unavail[source], strings.Join(misses[source], " "))
	}
	langs := make([]string, 0, len(byLang))
	for l := range byLang {
		langs = append(langs, l)
	}
	sort.Strings(langs)
	for _, l := range langs {
		var parts []string
		for _, src := range qualitySources {
			if v := byLang[l][src[0]]; v != nil {
				parts = append(parts, fmt.Sprintf("%s %d/%d/%d", src[0], v.h1, v.h5, v.n))
			}
		}
		t.Logf("  lang %s: %s   (hit@1/hit@5/questions)", l, strings.Join(parts, " · "))
	}
	t.Logf("overall hit@5 %d/%d of (question, source) pairs; any source %d/%d questions; docs version %s",
		pooled, total, answered, len(rows), qualityDocVersion)
	t.Logf("warm latency per query: p50 %.2f s, p95 %.2f s, max %.2f s over %d questions",
		percentile(times, .5).Seconds(), percentile(times, .95).Seconds(), slices.Max(times).Seconds(), len(rows))

	for i, set := range qualitySets {
		var parts []string
		for _, src := range qualitySources {
			if v := bySet[i][src[0]]; v != nil {
				parts = append(parts, fmt.Sprintf("%s %d/%d/%d", src[0], v.h1, v.h5, v.n))
			}
		}
		t.Logf("  set %s: %s   (hit@1/hit@5/questions)", set.name, strings.Join(parts, " · "))
	}

	for i, set := range qualitySets {
		for _, src := range qualitySources {
			source := src[0]
			h, floor := bySet[i][source], set.floor[source]
			if h == nil {
				h = &tally{}
			}
			if h.n != floor[2] {
				t.Errorf("%s %s: %d questions expect it, the floor was set on %d (fixture changed?)", set.name, source, h.n, floor[2])
				continue
			}
			for _, m := range []struct {
				name      string
				got, want int
			}{{"hit@1", h.h1, floor[0]}, {"hit@5", h.h5, floor[1]}} {
				switch {
				case m.got < m.want:
					t.Errorf("%s %s: %s %d/%d regressed below the floor %d/%d", set.name, source, m.name, m.got, h.n, m.want, h.n)
				case m.got > m.want:
					t.Logf("%s %s: %s %d/%d improved on the floor %d/%d; raise qualitySets", set.name, source, m.name, m.got, h.n, m.want, h.n)
				}
			}
		}
	}
}

// TestQualityFixtureMatchesResearch keeps the copied question set equal to niagara-research
// tools/tests/fixtures/golden-queries.jsonl while that file exists (skipped without a checkout).
func TestQualityFixtureMatchesResearch(t *testing.T) {
	// Opt-in like the other real-corpus checks (make parity sets it): the live
	// checkout moves independently of this branch, so make check must not read it.
	root := os.Getenv(QualityRootEnv)
	if root == "" {
		t.Skip(QualityRootEnv + " unset: the fixture is compared with the live checkout only under make parity")
	}
	research, err := os.ReadFile(filepath.Join(root, "tools", "tests", "fixtures", "golden-queries.jsonl"))
	if os.IsNotExist(err) {
		t.Skipf("no niagara-research question set under %s", root)
	}
	if err != nil {
		t.Fatal(err)
	}
	ours, err := os.ReadFile(qualityFixture)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(ours, research) {
		t.Errorf("%s differs from %s/tools/tests/fixtures/golden-queries.jsonl: copy it again", qualityFixture, root)
	}
}
