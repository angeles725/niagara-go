package modulefind

import (
	"github.com/angeles725/niagara-go/internal/pyre"
)

var (
	classKwRe     = pyre.MustCompile(`\bclass\s+`, 0)
	extendsTailRe = pyre.MustCompile(`\s+extends\s+(\w+)`, 0)
	classRunes    = []rune("class")
	dotSetRunes   = []rune(".set(")
	setRunes      = []rune("set")
)

// indexFrom is the first index >= from where sub occurs in text, or -1.
func indexFrom(text, sub []rune, from int) int {
	for i := from; i+len(sub) <= len(text); i++ {
		if text[i] == sub[0] && hasPrefixAt(text, sub, i) {
			return i
		}
	}
	return -1
}

func hasPrefixAt(text, sub []rune, i int) bool {
	if i+len(sub) > len(text) {
		return false
	}
	for k, r := range sub {
		if text[i+k] != r {
			return false
		}
	}
	return true
}

// extendsOf is group 1 of `\bclass\s+<escape(cls)>\s+extends\s+(\w+)` searched over
// text, without compiling that pattern per file. A match starts at an occurrence of
// "class"; there the greedy \s+ ends at the first non-space, and when cls starts with a
// non-space the literal can begin nowhere else, so the match exists iff cls follows that
// \s+ run and the tail matches right after it, with the same groups. Leftmost wins, so
// occurrences are tried in order. An empty cls, or one starting with a \s character,
// breaks that argument and takes the regex.
func extendsOf(text []rune, cls string) (string, bool) {
	lit := []rune(cls)
	if len(lit) == 0 || pyre.IsSpace(lit[0]) {
		if mt := pyre.MustCompile(`\bclass\s+`+escape(cls)+`\s+extends\s+(\w+)`, 0).SearchRunes(text, 0); mt != nil {
			return mt.Group(1), true
		}
		return "", false
	}
	for i := indexFrom(text, classRunes, 0); i >= 0; i = indexFrom(text, classRunes, i+1) {
		kw := classKwRe.MatchRunes(text, i)
		if kw == nil || !hasPrefixAt(text, lit, kw.End()) {
			continue
		}
		if t := extendsTailRe.MatchRunes(text, kw.End()+len(lit)); t != nil {
			return t.Group(1), true
		}
	}
	return "", false
}

// dynSetMatches is dynSetRe.FindIter(text) without trying the leading (\w+) at every
// position. Each match puts ".set(" right after its (\w+), and since '.' is not \w that
// group must run to the end of the \w run before ".set(" (backtracking cannot help). So
// for an occurrence at i whose \w run starts at a, every start in [max(a, lastEnd), i)
// matches or fails alike, and the leftmost one is max(a, lastEnd); runs of later
// occurrences lie after i, so occurrences in order give matches in order. Matches are
// never empty, so the next search resumes at the previous end, as finditer does.
func dynSetMatches(text []rune) []*pyre.Match {
	var out []*pyre.Match
	lastEnd := 0
	for i := indexFrom(text, dotSetRunes, 0); i >= 0; {
		s := i
		for s > lastEnd && pyre.IsWord(text[s-1]) {
			s--
		}
		if s < i {
			if mt := dynSetRe.MatchRunes(text, s); mt != nil {
				out = append(out, mt)
				lastEnd = mt.End()
				i = indexFrom(text, dotSetRunes, lastEnd)
				continue
			}
		}
		i = indexFrom(text, dotSetRunes, i+1)
	}
	return out
}

// findIterFrom is re.FindIter(text) for a pattern whose every match is non-empty and
// begins with the literal lit (lookarounds before it allowed): a match can start only
// where lit occurs, so trying re anchored at each occurrence at or after the previous
// end, in order, finds the same matches without the engine trying every position.
func findIterFrom(re *pyre.Regexp, lit []rune, text []rune) []*pyre.Match {
	var out []*pyre.Match
	for i := indexFrom(text, lit, 0); i >= 0; {
		if mt := re.MatchRunes(text, i); mt != nil {
			out = append(out, mt)
			i = indexFrom(text, lit, mt.End())
			continue
		}
		i = indexFrom(text, lit, i+1)
	}
	return out
}
