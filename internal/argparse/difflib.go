package argparse

// A port of difflib.get_close_matches(word, possibilities, n=1, cutoff=0.6), which
// argparse uses for "maybe you meant" suggestions. Sequences are code points.

type seqMatcher struct {
	a, b    []rune
	b2j     map[rune][]int
	bpopIdx map[rune]bool
}

func newSeqMatcher(b []rune) *seqMatcher {
	m := &seqMatcher{b: b, b2j: map[rune][]int{}, bpopIdx: map[rune]bool{}}
	for i, r := range b {
		m.b2j[r] = append(m.b2j[r], i)
	}
	if n := len(b); n >= 200 { // autojunk: drop elements more frequent than 1% + 1
		ntest := n/100 + 1
		for r, idx := range m.b2j {
			if len(idx) > ntest {
				m.bpopIdx[r] = true
				delete(m.b2j, r)
			}
		}
	}
	return m
}

// findLongestMatch is SequenceMatcher.find_longest_match with no junk function.
func (m *seqMatcher) findLongestMatch(alo, ahi, blo, bhi int) (int, int, int) {
	a, b := m.a, m.b
	besti, bestj, bestsize := alo, blo, 0
	j2len := map[int]int{}
	for i := alo; i < ahi; i++ {
		newj2len := map[int]int{}
		for _, j := range m.b2j[a[i]] {
			if j < blo {
				continue
			}
			if j >= bhi {
				break
			}
			k := j2len[j-1] + 1
			newj2len[j] = k
			if k > bestsize {
				besti, bestj, bestsize = i-k+1, j-k+1, k
			}
		}
		j2len = newj2len
	}
	// bjunk is empty, so both extension loops reduce to matching neighbours.
	for besti > alo && bestj > blo && a[besti-1] == b[bestj-1] {
		besti, bestj, bestsize = besti-1, bestj-1, bestsize+1
	}
	for besti+bestsize < ahi && bestj+bestsize < bhi && a[besti+bestsize] == b[bestj+bestsize] {
		bestsize++
	}
	return besti, bestj, bestsize
}

// matches is the total size of SequenceMatcher.get_matching_blocks().
func (m *seqMatcher) matches() int {
	type span struct{ alo, ahi, blo, bhi int }
	queue := []span{{0, len(m.a), 0, len(m.b)}}
	total := 0
	for len(queue) > 0 {
		q := queue[len(queue)-1]
		queue = queue[:len(queue)-1]
		i, j, k := m.findLongestMatch(q.alo, q.ahi, q.blo, q.bhi)
		if k == 0 {
			continue
		}
		total += k
		if q.alo < i && q.blo < j {
			queue = append(queue, span{q.alo, i, q.blo, j})
		}
		if i+k < q.ahi && j+k < q.bhi {
			queue = append(queue, span{i + k, q.ahi, j + k, q.bhi})
		}
	}
	return total
}

func ratio(matches, la, lb int) float64 {
	if la+lb == 0 {
		return 1.0
	}
	return 2.0 * float64(matches) / float64(la+lb)
}

// quickRatio is SequenceMatcher.quick_ratio: the multiset intersection size.
func (m *seqMatcher) quickRatio() float64 {
	full := map[rune]int{}
	for _, r := range m.b {
		full[r]++
	}
	avail := map[rune]int{}
	n := 0
	for _, r := range m.a {
		num, ok := avail[r]
		if !ok {
			num = full[r]
		}
		avail[r] = num - 1
		if num > 0 {
			n++
		}
	}
	return ratio(n, len(m.a), len(m.b))
}

// closeMatch returns get_close_matches(word, possibilities, 1)[0] and whether one exists.
func closeMatch(word string, possibilities []string) (string, bool) {
	const cutoff = 0.6
	m := newSeqMatcher([]rune(word))
	best, bestScore, found := "", 0.0, false
	for _, x := range possibilities {
		m.a = []rune(x)
		la, lb := len(m.a), len(m.b)
		lo := la
		if lb < lo {
			lo = lb
		}
		if ratio(lo, la, lb) < cutoff || m.quickRatio() < cutoff {
			continue
		}
		score := ratio(m.matches(), la, lb)
		if score < cutoff {
			continue
		}
		// heapq.nlargest(1, (score, x)): the highest score, ties to the larger string
		if !found || score > bestScore || (score == bestScore && x > best) {
			best, bestScore, found = x, score, true
		}
	}
	return best, found
}
