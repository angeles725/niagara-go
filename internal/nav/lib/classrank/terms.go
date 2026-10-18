// Package classrank is module-navigator class_rank.py and class_rank_cache.py:
// `rank <question>` (and `ask`, its other name) returns the top-level classes
// most relevant to a free-text question, English or Spanish, best first.
//
// Every top-level class of the class index is one document: its title is the
// class name split on camel case, its body the package, module, superclass and
// interfaces split the same way. Documents are scored with BM25 (bm25.ClassRank)
// times an importer prior, 1 + 0.3 * log1p(importers) from xref-index.json
// class_imported_by; a class named in the question ranks first. The prepared
// ranker is cached under indexes/.mnav-cache/ (cache.go).
package classrank

import (
	"strings"
	"sync"

	"github.com/angeles725/niagara-go/internal/glossary"
	"github.com/angeles725/niagara-go/internal/textfold"
)

// The ranking parameters of class_rank.py (K1 and B are bm25.ClassRank).
const (
	titleWeight = 8
	priorWeight = 0.3
)

// docMirror is DOC_MIRROR: Javadoc copies of classes that exist in their real module.
const docMirror = "docSource-doc"

// stopWords is class_rank.STOP_WORDS (not textsearch's list: it adds "about",
// "via", "hago", "funciona" and others).
var stopWords = func() map[string]bool {
	out := map[string]bool{}
	for _, w := range strings.Fields(`
a an and are as at be been but by can could do does did for from has have how i if in into is it its
may me my no not of on or our should so than that the their them then there these they this those to
too use used using vs was we what when where which who why will with would you your about also any all
each other such via
al como con cual cuales cuando de del donde el en entre es esta este esto estos estas hay la las le les
lo los mas me mi mis muy o para pero por porque que se si sin sobre son su sus tiene un una uno unos
unas y ya hago hace hacer funciona puedo`) {
		out[w] = true
	}
	return out
}()

// glossaryByStem is _GLOSSARY_BY_STEM.
var glossaryByStem = sync.OnceValue(glossary.ClassRankByStem)

// fold is class_rank.fold: NFKD, non-ASCII dropped, case kept.
func fold(s string) string { return textfold.Fold(s, textfold.ClassRank) }

func isUpper(c byte) bool { return 'A' <= c && c <= 'Z' }
func isLower(c byte) bool { return 'a' <= c && c <= 'z' }
func isDigit(c byte) bool { return '0' <= c && c <= '9' }

// parts is _PART_RE.findall: [A-Z]+(?![a-z]) | [A-Z]?[a-z]+ | [0-9]+, ASCII
// only (every other byte separates parts). An upper-case run followed by a
// lower-case letter gives its last capital to the next part ("BACnet" -> BA, Cnet).
func parts(s string) []string {
	var out []string
	for i := 0; i < len(s); {
		c := s[i]
		switch {
		case isUpper(c):
			j := i
			for j < len(s) && isUpper(s[j]) {
				j++
			}
			if j == len(s) || !isLower(s[j]) {
				out, i = append(out, s[i:j]), j
			} else if j-i >= 2 {
				out, i = append(out, s[i:j-1]), j-1
			} else {
				k := j
				for k < len(s) && isLower(s[k]) {
					k++
				}
				out, i = append(out, s[i:k]), k
			}
		case isLower(c), isDigit(c):
			class := isLower
			if isDigit(c) {
				class = isDigit
			}
			j := i
			for j < len(s) && class(s[j]) {
				j++
			}
			out, i = append(out, s[i:j]), j
		default:
			i++
		}
	}
	return out
}

// tokens is _TOKEN_RE.findall: the [A-Za-z0-9_]+ runs of s.
func tokens(s string) []string {
	var out []string
	start := -1
	for i := 0; i <= len(s); i++ {
		if i < len(s) && (isUpper(s[i]) || isLower(s[i]) || isDigit(s[i]) || s[i] == '_') {
			if start < 0 {
				start = i
			}
			continue
		}
		if start >= 0 {
			out, start = append(out, s[start:i]), -1
		}
	}
	return out
}

// stem is class_rank._stem (= textfold.Stem on the ASCII terms it sees).
func stem(t string) string { return textfold.Stem(t) }

func keep(t string) bool { return len(t) > 1 && !stopWords[t] }

// terms memoizes the term functions (class_rank's lru_caches): packages,
// modules and supertypes repeat across classes.
type terms struct {
	ident  map[string][]string
	text   map[string][]string
	counts map[string]map[string]int
}

func newTerms() *terms {
	return &terms{ident: map[string][]string{}, text: map[string][]string{},
		counts: map[string]map[string]int{}}
}

// identifier is identifier_terms: the camel-case parts of one identifier, each
// two adjacent parts joined ("BA"+"Cnet" -> "bacnet") and the whole identifier,
// lower case, stop-words and 1-char parts dropped (joins and the whole word
// keep them), stemmed, in order without repeats.
func (t *terms) identifier(id string) []string {
	if out, ok := t.ident[id]; ok {
		return out
	}
	ps := parts(id)
	for i, p := range ps {
		ps[i] = strings.ToLower(p)
	}
	var all []string
	for _, p := range ps {
		if keep(p) {
			all = append(all, p)
		}
	}
	for i := 0; i+1 < len(ps); i++ {
		if len(ps[i]) > 1 && len(ps[i+1]) > 1 {
			all = append(all, ps[i]+ps[i+1])
		}
	}
	if len(ps) > 1 {
		all = append(all, strings.Join(ps, ""))
	}
	seen := map[string]bool{}
	out := []string{}
	for _, a := range all {
		if s := stem(a); !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	t.ident[id] = out
	return out
}

// textTerms is _text_terms: identifier terms of every identifier-like word of
// the folded text, in order (repeats kept).
func (t *terms) textTerms(text string) []string {
	if out, ok := t.text[text]; ok {
		return out
	}
	out := []string{}
	for _, tok := range tokens(fold(text)) {
		out = append(out, t.identifier(tok)...)
	}
	t.text[text] = out
	return out
}

// fieldCounts is _field_counts: term -> occurrences in one body field.
func (t *terms) fieldCounts(text string) map[string]int {
	if out, ok := t.counts[text]; ok {
		return out
	}
	out := map[string]int{}
	for _, term := range t.textTerms(text) {
		out[term]++
	}
	t.counts[text] = out
	return out
}

// partCount is _part_count: camel-case parts of the folded text.
func partCount(text string) int { return len(parts(fold(text))) }

// squash is _squash: folded, lower case, no underscores; every ranking term of
// the text is a substring of it (the prefilter of a query term).
func squash(text string) string {
	return strings.ReplaceAll(strings.ToLower(fold(text)), "_", "")
}

// queryTerms is query_terms: the distinct ranking terms of a question in
// question order, a Spanish glossary word replaced by its English terms.
func (t *terms) queryTerms(question string) []string {
	g := glossaryByStem()
	seen := map[string]bool{}
	out := []string{}
	for _, term := range t.textTerms(question) {
		repl := g[term]
		if len(repl) == 0 {
			repl = []string{term}
		}
		for _, r := range repl {
			if !seen[r] {
				seen[r] = true
				out = append(out, r)
			}
		}
	}
	return out
}

// isIdentifier is _IDENTIFIER_RE on a token: a letter first and a capital
// after it ("ackAlarm", "BWebServlet"; not "Alarm").
func isIdentifier(tok string) bool {
	if tok == "" || !(isUpper(tok[0]) || isLower(tok[0])) {
		return false
	}
	for i := 1; i < len(tok); i++ {
		if isUpper(tok[i]) {
			return true
		}
	}
	return false
}
