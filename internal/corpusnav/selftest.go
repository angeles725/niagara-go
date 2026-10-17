package corpusnav

import (
	"fmt"
	"strings"

	"github.com/angeles725/niagara-go/internal/bm25"
	"github.com/angeles725/niagara-go/internal/pycompat"
	"github.com/angeles725/niagara-go/internal/textfold"
)

// cmdSelftest is corpusnav/selftest.py: assertions on the live corpus and on the ranking
// engines; failures go to stderr and exit 1.
func (c *corpus) cmdSelftest() int {
	var errs []string
	index := c.newMemIndex(c.targets("blocks"))
	for _, t := range []struct {
		label, query string
		all          bool
	}{{"single-word", "BStatus", false}, {"two-word OR", "BStatus overridden", false},
		{"two-word AND", "BStatus overridden", true}} {
		if hits := len(index.rank(t.query, t.all, nil)); hits == 0 {
			errs = append(errs, fmt.Sprintf("%s (%s) → 0 blocks; expected > 0", pycompat.StrRepr(t.query), t.label))
		} else {
			fmt.Fprintf(c.out, "  PASS  %s %s → %d block(s)\n", t.label, pycompat.StrRepr(t.query), hits)
		}
	}

	// the cached find index ranks exactly like the in-memory BM25
	query := "BStatus overridden"
	stored := c.ranked("blocks", query, false, nil)
	mem := index.rank(query, false, nil)
	same := len(stored.results) == len(mem)
	for i := 0; same && i < len(mem); i++ {
		same = stored.results[i].score == mem[i].score && stored.results[i].doc.Display == mem[i].doc.Display
	}
	if !same {
		errs = append(errs, fmt.Sprintf("find index: %s ranks differently from the in-memory BM25", pycompat.StrRepr(query)))
	} else {
		how := "warm"
		if stored.fallback != "" {
			how = stored.fallback + ", rebuilt"
		}
		fmt.Fprintf(c.out, "  PASS  find index (%s) ranks %s like memory → %d block(s)\n", how, pycompat.StrRepr(query), len(mem))
	}

	// ranking: the block ABOUT the topic comes first (synthetic, corpus-independent)
	bm := bm25.NewIndex([]bm25.Doc[string]{{Key: "about", Title: "Alarm fault handling", Body: "alarm fault"},
		{Key: "mention", Title: "Unrelated", Body: "alarm " + strings.Repeat("filler ", 200)}}, bm25.Textsearch, 8)
	var order []string
	for _, h := range bm.Rank("alarm faults", false, nil) {
		order = append(order, h.Key)
	}
	if got := pyList(order); got != "['about', 'mention']" {
		errs = append(errs, fmt.Sprintf("ranking: BM25 order %s, expected ['about', 'mention']", got))
	} else {
		fmt.Fprintf(c.out, "  PASS  ranking: titled block outranks a long mention %s\n", got)
	}

	// semantic ranking puts a status/BStatus block in the top 5
	catalog := c.loadCatalog()
	top := semanticRanked("point health fault override status", c.blocks(), nil)
	top = top[:min(5, len(top))]
	var ids, titles []string
	ok := false
	for _, h := range top {
		ids = append(ids, displayID(h.b.label))
		t := textfold.Lower(titleFor(h.b.label, catalog, h.b.path))
		titles = append(titles, t)
		ok = ok || strings.Contains(t, "status") || strings.Contains(t, "bstatus")
	}
	if !ok {
		errs = append(errs, fmt.Sprintf("semantic 'point health fault override status': top-5 %s titles %s — "+
			"expected one mentioning status/BStatus", pyList(ids), pyList(titles)))
	} else {
		fmt.Fprintf(c.out, "  PASS  semantic ranking 'status' query → top-5 %s\n", pyList(ids))
	}

	if len(errs) > 0 {
		for _, e := range errs {
			fmt.Fprintf(c.err, "  FAIL  %s\n", e)
		}
		return 1
	}
	fmt.Fprintln(c.out, "selftest: all assertions passed.")
	return 0
}

// pyList is repr() of a list of str.
func pyList(xs []string) string {
	parts := make([]string, len(xs))
	for i, x := range xs {
		parts[i] = pycompat.StrRepr(x)
	}
	return "[" + strings.Join(parts, ", ") + "]"
}
