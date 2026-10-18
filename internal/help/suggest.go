package help

// "Did you mean" suggestions (fuzzy_search.py suggest, typo_similarity,
// edit_distance), with difflib.SequenceMatcher.quick_ratio as the cheap
// first filter. Lengths and edits count code points like Python str.

import (
	"sort"

	"github.com/angeles725/niagara-go/internal/difflib"
	"github.com/angeles725/niagara-go/internal/pycompat"
)

// editDistance is the Levenshtein distance of a and b, or limit+1 once it
// exceeds limit.
func editDistance(a, b []rune, limit int) int {
	if abs(len(a)-len(b)) > limit {
		return limit + 1
	}
	prev := make([]int, len(b)+1)
	for j := range prev {
		prev[j] = j
	}
	for i, ca := range a {
		cur := make([]int, 1, len(b)+1)
		cur[0] = i + 1
		low := cur[0]
		for j, cb := range b {
			sub := prev[j]
			if ca != cb {
				sub++
			}
			v := min(prev[j+1]+1, cur[j]+1, sub)
			cur = append(cur, v)
			low = min(low, v)
		}
		if low > limit {
			return limit + 1
		}
		prev = cur
	}
	return prev[len(prev)-1]
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

// typoSimilarity is 1 - distance/length when name is word with a typo (at
// most max(2, len/5) edits), else 0.
func typoSimilarity(word, name string) float64 {
	w, n := []rune(word), []rune(name)
	limit := max(2, len(w)/5)
	if abs(len(w)-len(n)) > limit {
		return 0
	}
	if difflib.New(nil, w, n, true).QuickRatio() < 0.75 {
		return 0
	}
	d := editDistance(w, n, limit)
	if d > limit || max(len(w), len(n)) == 0 {
		// Two empty strings are not a typo of each other; Python divides
		// by zero there (ZeroDivisionError), Go would score NaN.
		return 0
	}
	return 1.0 - float64(d)/float64(max(len(w), len(n)))
}

// suggest returns up to n names that are query with a typo, closest first:
// sorted by (-similarity, length, name).
func suggest(names []string, query string, n int) []string {
	type scored struct {
		sim  float64
		size int
		name string
	}
	q := pycompat.Lower(query)
	var hits []scored
	for _, name := range names {
		if sim := typoSimilarity(q, pycompat.Lower(name)); sim != 0 {
			hits = append(hits, scored{sim, runeLen(name), name})
		}
	}
	sort.Slice(hits, func(i, j int) bool {
		a, b := hits[i], hits[j]
		if a.sim != b.sim {
			return a.sim > b.sim
		}
		if a.size != b.size {
			return a.size < b.size
		}
		return a.name < b.name
	})
	out := []string{}
	for i := 0; i < len(hits) && i < n; i++ {
		out = append(out, hits[i].name)
	}
	return out
}
