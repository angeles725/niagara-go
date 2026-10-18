package pyre

import (
	"sort"
	"sync"

	"github.com/dlclark/regexp2"
)

// IsWord reports whether r is in the str-pattern class \w (Python's re, Unicode).
func IsWord(r rune) bool { return inRanges(pyWordRanges, r) }

// IsSpace reports whether r is in the str-pattern class \s (Python's re, Unicode).
func IsSpace(r rune) bool { return inRanges(pySpaceRanges, r) }

// inRanges binary-searches inclusive (lo, hi) pairs sorted by lo.
func inRanges(pairs []rune, r rune) bool {
	n := len(pairs) / 2
	i := sort.Search(n, func(i int) bool { return pairs[2*i+1] >= r })
	return i < n && pairs[2*i] <= r
}

// anchoredProgram compiles \G(?:body) lazily: \G holds only at the start position, so
// regexp2 tries the body there and nowhere else. nil when it does not compile.
func anchoredProgram(body string, opts regexp2.RegexOptions) func() *regexp2.Regexp {
	return sync.OnceValue(func() *regexp2.Regexp {
		re, err := regexp2.Compile(`\G(?:`+body+`)`, opts)
		if err != nil {
			return nil
		}
		return re
	})
}

// MatchRunes is pattern.match(text, pos): a match starting exactly at pos, or nil.
// Lookbehinds and \b see the text before pos; ^ without MULTILINE (and \A) hold only at
// 0, as in Python.
func (r *Regexp) MatchRunes(text []rune, pos int) *Match {
	if r.anchored != nil {
		if re := r.anchored(); re != nil {
			m, err := re.FindRunesMatchStartingAt(text, pos)
			if err != nil || m == nil {
				return nil
			}
			return &Match{m: m, re: r}
		}
	}
	// same answer, slower: the leftmost match from pos, kept only when it starts at pos
	if m := r.SearchRunes(text, pos); m != nil && m.Start() == pos {
		return m
	}
	return nil
}
