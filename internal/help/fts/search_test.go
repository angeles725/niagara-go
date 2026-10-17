package fts

import (
	"encoding/json"
	"errors"
	"math"
	"os"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/angeles725/niagara-go/internal/help/fts/ftsfixture"
)

// searchFixture is testdata/fulltext-search.json (fulltext_gen.py): Python's
// fulltext.search results over the fixture checkout and fulltext.clip cases.
type searchFixture struct {
	Searches []struct {
		Version  string     `json:"version"`
		Query    string     `json:"query"`
		Sources  []string   `json:"sources"`
		Mode     string     `json:"mode"`
		Groups   [][]string `json:"groups"`
		ModeUsed string     `json:"mode_used"`
		Fallback bool       `json:"fallback"`
		AndHits  int        `json:"and_hits"`
		Hits     [][]any    `json:"hits"` // key, score (float.hex), title, line, page, coverage, [line, snippet]
	} `json:"searches"`
	Clips [][3]string `json:"clips"` // line, query, clip(line, parse_query(query), 40)
}

func loadSearchFixture(t *testing.T) searchFixture {
	t.Helper()
	raw, err := os.ReadFile(testdata + "/fulltext-search.json")
	if err != nil {
		t.Fatal(err)
	}
	var f searchFixture
	if err := json.Unmarshal(raw, &f); err != nil {
		t.Fatal(err)
	}
	return f
}

// hitRecord renders a Go hit like the fixture's records.
func hitRecord(h *Hit, n int, snippet string) []any {
	var line any
	if n != 0 {
		line = float64(n)
	}
	return []any{h.Key(), strconv.FormatFloat(h.Score, 'x', -1, 64), h.Title, float64(h.Line),
		float64(h.Page), float64(h.Coverage), []any{line, snippet}}
}

// pyHex normalizes Python's float.hex ("0x1.d81f17d4d019dp+1") to Go's 'x'
// format ("0x1.d81f17d4d019dp+01").
func pyHex(s string) string {
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return s
	}
	return strconv.FormatFloat(f, 'x', -1, 64)
}

func TestSearchMatchesPython(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	checkout := ftsfixture.Checkout(t, testdata, false)
	for _, c := range loadSearchFixture(t).Searches {
		t.Run(c.Version+" "+c.Query+" "+strings.Join(c.Sources, ",")+" "+c.Mode, func(t *testing.T) {
			base := dataRoot(checkout, c.Version)
			r, err := Search(base, c.Version, c.Query, c.Sources, c.Mode)
			if err != nil {
				t.Fatal(err)
			}
			if got := groupsOf(r.Groups); !reflect.DeepEqual(got, c.Groups) {
				t.Errorf("groups %q, want %q", got, c.Groups)
			}
			if r.ModeUsed != c.ModeUsed || r.Fallback != c.Fallback || r.AndHits != c.AndHits {
				t.Errorf("mode %s fallback %v and_hits %d, want %s %v %d",
					r.ModeUsed, r.Fallback, r.AndHits, c.ModeUsed, c.Fallback, c.AndHits)
			}
			dirs := SourceDirs(base)
			got := [][]any{}
			for _, h := range r.Hits {
				n, text := Snippet(dirs, h, r.Groups)
				got = append(got, hitRecord(h, n, text))
			}
			want := c.Hits
			for _, h := range want {
				h[1] = pyHex(h[1].(string))
			}
			if !reflect.DeepEqual(got, orEmpty(want)) {
				t.Errorf("hits:\n got %v\nwant %v", got, want)
			}
		})
	}
}

func TestClipMatchesPython(t *testing.T) {
	for _, c := range loadSearchFixture(t).Clips {
		if got := Clip(c[0], ParseQuery(c[1], false), 40); got != c[2] {
			t.Errorf("Clip(%q, %q) = %q, want %q", c[0], c[1], got, c[2])
		}
	}
}

func TestSearchRejectsAQueryWithoutWords(t *testing.T) {
	var usage *UsageError
	_, err := Search(t.TempDir(), "", "!!! ???", nil, "and")
	if !errors.As(err, &usage) || err.Error() != "query '!!! ???' has no searchable word" {
		t.Errorf("err = %v", err)
	}
}

func TestSearchWithoutSourcesNeedsNoIndex(t *testing.T) {
	r, err := Search(t.TempDir(), "", "alarm", nil, "and")
	if err != nil || len(r.Hits) != 0 || r.ModeUsed != "and" {
		t.Errorf("r = %+v, err = %v", r, err)
	}
}

func TestRankAggregatesPagesLikePython(t *testing.T) {
	rows := []row{
		{id: 1, source: "docs-text", path: "m.txt", title: "M", line: 1, page: 1, bm25: -1},
		{id: 2, source: "docs-text", path: "m.txt", title: "M", line: 9, page: 2, bm25: -3},
		{id: 3, source: "source", path: "B.java", title: "B", line: 1, bm25: -4},
		{id: 4, source: "devguide-clean", path: "changes30.txt", title: "C", line: 1, bm25: -8},
	}
	hits := rank(rows, func(int64) int { return 1 }, 2)
	keys := []string{}
	for _, h := range hits {
		keys = append(keys, h.Key())
	}
	// coverage 1 of 2 groups: m.txt (3 + 1 x 0.85) / 4, B.java 4 / 4 x 0.5,
	// changes30 8 / 4 x 0.25.
	if want := []string{"docs-text/m.txt", "devguide-clean/changes30.txt", "source/B.java"}; !reflect.DeepEqual(keys, want) {
		t.Fatalf("order %q, want %q", keys, want)
	}
	if hits[0].Page != 2 || hits[0].Line != 9 || math.Abs(hits[0].Score-(0.75+0.25*0.85)) > 1e-15 {
		t.Errorf("best page: %+v", hits[0])
	}
	// The code file and the change log tie at 0.5: the source label decides.
	if hits[1].Score != 0.5 || hits[2].Score != 0.5 {
		t.Errorf("tie scores %v %v, want 0.5 0.5", hits[1].Score, hits[2].Score)
	}
}

func TestNavAndHeaderLines(t *testing.T) {
	for line, want := range map[string]bool{
		"Prev | Next": true, "Index | Prev | Next": true, "Index |Prev\t|  Next": true,
		"Index | Next": false, "Prev | Next more": false, "PrevNext": false, "Prev |": false,
	} {
		if got := isNavLine(line); got != want {
			t.Errorf("isNavLine(%q) = %v", line, got)
		}
	}
	lines := []string{"TITLE: X", "SOURCE NAME: y", " ", "body"}
	if got := bodyStart(lines); got != 3 {
		t.Errorf("bodyStart = %d, want 3", got)
	}
	if got := bodyStart([]string{"Title: no", "TITLE: late"}); got != 0 {
		t.Errorf("bodyStart without a header = %d", got)
	}
	if isHeaderLine("A B") || !isHeaderLine("A B:") || isHeaderLine(":x") {
		t.Error("isHeaderLine")
	}
}
