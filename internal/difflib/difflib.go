// Package difflib ports the parts of Python 3.15 difflib the tools use: SequenceMatcher
// (find_longest_match, get_matching_blocks, ratio, quick_ratio, real_quick_ratio, with the
// isjunk and autojunk rules of CPython) and get_close_matches. Sequences are slices of any
// comparable element; strings are compared by code point (Runes).
package difflib

import (
	"errors"
	"fmt"
	"sort"
)

// Match is difflib.Match: a[A:A+Size] == b[B:B+Size].
type Match struct{ A, B, Size int }

// SequenceMatcher is difflib.SequenceMatcher(isjunk, a, b, autojunk).
type SequenceMatcher[T comparable] struct {
	a, b     []T
	isJunk   func(T) bool
	autojunk bool
	b2j      map[T][]int
	bjunk    map[T]bool
	bpopular map[T]bool
	full     map[T]int // fullbcount, built on the first QuickRatio
	blocks   []Match
}

// New is SequenceMatcher(isjunk, a, b, autojunk); isJunk may be nil (Python None).
func New[T comparable](isJunk func(T) bool, a, b []T, autojunk bool) *SequenceMatcher[T] {
	m := &SequenceMatcher[T]{isJunk: isJunk, autojunk: autojunk}
	m.SetSeqs(a, b)
	return m
}

// Runes is the element sequence of a Python str.
func Runes(s string) []rune { return []rune(s) }

// SetSeqs is set_seqs(a, b).
func (m *SequenceMatcher[T]) SetSeqs(a, b []T) {
	m.SetSeq1(a)
	m.SetSeq2(b)
}

// SetSeq1 is set_seq1(a): b's index is kept.
func (m *SequenceMatcher[T]) SetSeq1(a []T) {
	m.a, m.blocks = a, nil
}

// SetSeq2 is set_seq2(b): b's index (b2j, junk and popular elements) is rebuilt.
func (m *SequenceMatcher[T]) SetSeq2(b []T) {
	m.b, m.blocks, m.full = b, nil, nil
	m.chainB()
}

// chainB is __chain_b.
func (m *SequenceMatcher[T]) chainB() {
	m.b2j = map[T][]int{}
	for i, e := range m.b {
		m.b2j[e] = append(m.b2j[e], i)
	}
	m.bjunk = map[T]bool{}
	if m.isJunk != nil {
		for e := range m.b2j {
			if m.isJunk(e) {
				m.bjunk[e] = true
			}
		}
		for e := range m.bjunk {
			delete(m.b2j, e)
		}
	}
	m.bpopular = map[T]bool{}
	if n := len(m.b); m.autojunk && n >= 200 {
		ntest := n/100 + 1
		for e, idx := range m.b2j {
			if len(idx) > ntest {
				m.bpopular[e] = true
			}
		}
		for e := range m.bpopular {
			delete(m.b2j, e)
		}
	}
}

// FindLongestMatch is find_longest_match(alo, ahi, blo, bhi): the longest non-junk match,
// earliest in a, then earliest in b, extended by matching junk on both sides.
func (m *SequenceMatcher[T]) FindLongestMatch(alo, ahi, blo, bhi int) Match {
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
	for besti > alo && bestj > blo && !m.bjunk[b[bestj-1]] && a[besti-1] == b[bestj-1] {
		besti, bestj, bestsize = besti-1, bestj-1, bestsize+1
	}
	for besti+bestsize < ahi && bestj+bestsize < bhi && !m.bjunk[b[bestj+bestsize]] &&
		a[besti+bestsize] == b[bestj+bestsize] {
		bestsize++
	}
	for besti > alo && bestj > blo && m.bjunk[b[bestj-1]] && a[besti-1] == b[bestj-1] {
		besti, bestj, bestsize = besti-1, bestj-1, bestsize+1
	}
	for besti+bestsize < ahi && bestj+bestsize < bhi && m.bjunk[b[bestj+bestsize]] &&
		a[besti+bestsize] == b[bestj+bestsize] {
		bestsize++
	}
	return Match{besti, bestj, bestsize}
}

// GetMatchingBlocks is get_matching_blocks(): non-adjacent matches in order, ending with
// the (len(a), len(b), 0) sentinel. The returned slice must not be modified.
func (m *SequenceMatcher[T]) GetMatchingBlocks() []Match {
	if m.blocks != nil {
		return m.blocks
	}
	la, lb := len(m.a), len(m.b)
	type span struct{ alo, ahi, blo, bhi int }
	queue := []span{{0, la, 0, lb}}
	var found []Match
	for len(queue) > 0 {
		q := queue[len(queue)-1]
		queue = queue[:len(queue)-1]
		x := m.FindLongestMatch(q.alo, q.ahi, q.blo, q.bhi)
		if x.Size == 0 {
			continue
		}
		found = append(found, x)
		if q.alo < x.A && q.blo < x.B {
			queue = append(queue, span{q.alo, x.A, q.blo, x.B})
		}
		if x.A+x.Size < q.ahi && x.B+x.Size < q.bhi {
			queue = append(queue, span{x.A + x.Size, q.ahi, x.B + x.Size, q.bhi})
		}
	}
	sort.Slice(found, func(i, j int) bool {
		if found[i].A != found[j].A {
			return found[i].A < found[j].A
		}
		if found[i].B != found[j].B {
			return found[i].B < found[j].B
		}
		return found[i].Size < found[j].Size
	})
	var out []Match
	cur := Match{}
	for _, x := range found {
		if cur.A+cur.Size == x.A && cur.B+cur.Size == x.B {
			cur.Size += x.Size
			continue
		}
		if cur.Size > 0 {
			out = append(out, cur)
		}
		cur = x
	}
	if cur.Size > 0 {
		out = append(out, cur)
	}
	m.blocks = append(out, Match{la, lb, 0})
	return m.blocks
}

func ratio(matches, length int) float64 {
	if length == 0 {
		return 1.0
	}
	return 2.0 * float64(matches) / float64(length)
}

// Ratio is ratio(): 2*M/T over the matching blocks.
func (m *SequenceMatcher[T]) Ratio() float64 {
	n := 0
	for _, x := range m.GetMatchingBlocks() {
		n += x.Size
	}
	return ratio(n, len(m.a)+len(m.b))
}

// QuickRatio is quick_ratio(): an upper bound on Ratio from element counts.
func (m *SequenceMatcher[T]) QuickRatio() float64 {
	if m.full == nil {
		m.full = map[T]int{}
		for _, e := range m.b {
			m.full[e]++
		}
	}
	avail := map[T]int{}
	n := 0
	for _, e := range m.a {
		num, ok := avail[e]
		if !ok {
			num = m.full[e]
		}
		avail[e] = num - 1
		if num > 0 {
			n++
		}
	}
	return ratio(n, len(m.a)+len(m.b))
}

// RealQuickRatio is real_quick_ratio(): an upper bound on Ratio from the lengths.
func (m *SequenceMatcher[T]) RealQuickRatio() float64 {
	la, lb := len(m.a), len(m.b)
	return ratio(min(la, lb), la+lb)
}

// ErrBadArgument is the ValueError of get_close_matches for n <= 0 or cutoff outside [0, 1].
var ErrBadArgument = errors.New("difflib: bad argument")

// GetCloseMatches is get_close_matches(word, possibilities, n, cutoff): up to n
// possibilities whose ratio to word is at least cutoff, best first (ties: larger string).
func GetCloseMatches(word string, possibilities []string, n int, cutoff float64) ([]string, error) {
	if n <= 0 {
		return nil, fmt.Errorf("%w: n must be > 0: %d", ErrBadArgument, n)
	}
	if !(cutoff >= 0 && cutoff <= 1) {
		return nil, fmt.Errorf("%w: cutoff must be in [0.0, 1.0]: %v", ErrBadArgument, cutoff)
	}
	type scored struct {
		score float64
		x     string
	}
	var result []scored
	s := New[rune](nil, nil, Runes(word), true)
	for _, x := range possibilities {
		s.SetSeq1(Runes(x))
		if s.RealQuickRatio() < cutoff || s.QuickRatio() < cutoff {
			continue
		}
		if r := s.Ratio(); r >= cutoff {
			result = append(result, scored{r, x})
		}
	}
	// heapq.nlargest(n, result): descending (score, x) tuples
	sort.SliceStable(result, func(i, j int) bool {
		if result[i].score != result[j].score {
			return result[i].score > result[j].score
		}
		return result[i].x > result[j].x
	})
	if len(result) > n {
		result = result[:n]
	}
	out := make([]string, len(result))
	for i, r := range result {
		out[i] = r.x
	}
	return out, nil
}
