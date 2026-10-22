package fts

// The tokenizer and query parser (fulltext_terms.py): words split on
// non-alphanumerics and camelCase, lower-cased, accent-folded and lightly
// stemmed; a query word matches whole tokens only.

import (
	"fmt"
	"slices"
	"sort"
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/angeles725/niagara-go/internal/glossary"
	"github.com/angeles725/niagara-go/internal/pycompat"
	"github.com/angeles725/niagara-go/internal/textfold"
)

// Fold is fulltext_terms.fold: accents stripped, case kept, "°" spelled " deg".
func Fold(text string) string { return textfold.Fold(text, textfold.HelpFulltext) }

// word is one match of fulltext_terms._WORD (`[^\W_]+`) and its start as a
// code-point offset, what re.finditer's m.start() reports.
type word struct {
	text  string
	start int
}

// isWordChar is `[^\W_]`: re's Unicode \w without the underscore, which is
// str.isalnum().
func isWordChar(r rune) bool { return r != '_' && pycompat.IsWordRune(r) }

// scanWords is _WORD.finditer(s).
func scanWords(s string) []word {
	var out []word
	begin, beginRune, n := -1, 0, 0
	for i, r := range s {
		if isWordChar(r) {
			if begin < 0 {
				begin, beginRune = i, n
			}
		} else if begin >= 0 {
			out = append(out, word{s[begin:i], beginRune})
			begin = -1
		}
		n++
	}
	if begin >= 0 {
		out = append(out, word{s[begin:], beginRune})
	}
	return out
}

// Words is _WORD.findall(s).
func Words(s string) []string {
	ws := scanWords(s)
	out := make([]string, len(ws))
	for i, w := range ws {
		out[i] = w.text
	}
	return out
}

func isUpper(b byte) bool { return 'A' <= b && b <= 'Z' }
func isLower(b byte) bool { return 'a' <= b && b <= 'z' }
func isDigit(b byte) bool { return '0' <= b && b <= '9' }

// run is the length of the run of bytes satisfying f from i.
func run(s string, i int, f func(byte) bool) int {
	j := i
	for j < len(s) && f(s[j]) {
		j++
	}
	return j - i
}

// camelParts is _CAMEL.findall(s) for
// `[A-Z]+(?=[A-Z][a-z])|[A-Z]?[a-z]+|[A-Z]+|[0-9]+`: at each position the
// first alternative that matches wins, else the scan moves one character on.
// Every class is ASCII, so a byte scan is exact (a non-ASCII byte matches
// nothing). The lookahead alternative can only stop one letter before the end
// of an upper-case run that a lower-case letter follows.
func camelParts(s string) []string {
	var out []string
	for i := 0; i < len(s); {
		up := run(s, i, isUpper)
		var n int
		switch {
		case up >= 2 && i+up < len(s) && isLower(s[i+up]):
			n = up - 1
		case up >= 1 && run(s, i+1, isLower) > 0:
			n = 1 + run(s, i+1, isLower)
		case up == 0 && run(s, i, isLower) > 0:
			n = run(s, i, isLower)
		case up > 0:
			n = up
		default:
			n = run(s, i, isDigit)
		}
		if n == 0 {
			i++
			continue
		}
		out = append(out, s[i:i+n])
		i += n
	}
	return out
}

// suffixes are fulltext_terms._SUFFIXES: first match wins, the stem keeps
// three letters or more.
var suffixes = [][2]string{{"ies", "y"}, {"ations", ""}, {"ation", ""}, {"ings", ""},
	{"ing", ""}, {"ed", ""}, {"es", ""}, {"s", ""}}

// isAlpha is str.isalpha(): non-empty and every character a letter.
func isAlpha(s string) bool {
	for _, r := range s {
		if !pycompat.IsAlpha(r) {
			return false
		}
	}
	return s != ""
}

// isDigitStr is str.isdigit(): non-empty and every character a digit
// (superscripts included).
func isDigitStr(s string) bool {
	for _, r := range s {
		if !pycompat.IsDigit(r) {
			return false
		}
	}
	return s != ""
}

// Stem is fulltext_terms.stem, the light English stemmer of the index and
// the query. Lengths count code points.
func Stem(w string) string {
	if utf8.RuneCountInString(w) < 4 || !isAlpha(w) {
		return w
	}
	for _, sr := range suffixes {
		suffix, repl := sr[0], sr[1]
		if strings.HasSuffix(w, suffix) && utf8.RuneCountInString(w)-len(suffix) >= 3 {
			if suffix == "s" && strings.HasSuffix(w, "ss") {
				break
			}
			w = w[:len(w)-len(suffix)] + repl
			break
		}
	}
	if strings.HasSuffix(w, "e") && utf8.RuneCountInString(w) >= 4 {
		w = w[:len(w)-1]
	}
	return w
}

var (
	wordTermsMu    sync.Mutex
	wordTermsCache = map[string][]string{}
)

// WordTerms is fulltext_terms.word_terms: the stemmed lower-case word, then
// its camelCase parts of two letters or more, each once. The result is
// cached and shared: never modify it.
func WordTerms(w string) []string {
	wordTermsMu.Lock()
	defer wordTermsMu.Unlock()
	if t, ok := wordTermsCache[w]; ok {
		return t
	}
	out := []string{Stem(textfold.Lower(w))}
	if parts := camelParts(w); len(parts) > 1 {
		for _, p := range parts {
			t := Stem(textfold.Lower(p))
			if utf8.RuneCountInString(t) >= 2 && !slices.Contains(out, t) {
				out = append(out, t)
			}
		}
	}
	wordTermsCache[w] = out
	return out
}

// Terms is fulltext_terms.terms: every index term of a text in order,
// duplicates kept. A one-character word counts only when it is a digit.
func Terms(text string) []string {
	var out []string
	for _, w := range Words(Fold(text)) {
		if utf8.RuneCountInString(w) >= 2 || isDigitStr(w) {
			out = append(out, WordTerms(w)...)
		}
	}
	return out
}

// TermSet is fulltext_terms.term_set.
func TermSet(text string) map[string]bool {
	set := map[string]bool{}
	for _, t := range Terms(text) {
		set[t] = true
	}
	return set
}

// helpGlossary is search_lang.GLOSSARY.
var helpGlossary = glossary.Help()

// Group is one query word: its alternative terms, sorted. An alternative
// ending in "*" is a prefix term, one holding spaces is a phrase.
type Group []string

func (g Group) equal(o Group) bool {
	if len(g) != len(o) {
		return false
	}
	for i := range g {
		if g[i] != o[i] {
			return false
		}
	}
	return true
}

// ParseQuery is fulltext_terms.parse_query: one group per distinct word,
// stop-words dropped unless nothing else is left, a Spanish glossary word
// replaced by its English terms, a stem that lost a suffix (five letters or
// more left) made a prefix term and, with relax, the camelCase parts added
// as a phrase.
func ParseQuery(query string, relax bool) []Group {
	var groups []Group
	for _, w := range queryWords(query) {
		low := textfold.Lower(w)
		english := helpGlossary.LookupKey(low)
		if len(english) == 0 {
			english = []string{low}
		}
		alts := map[string]bool{}
		for _, term := range english {
			stemmed := Stem(term)
			if stemmed != term && utf8.RuneCountInString(stemmed) >= 5 {
				stemmed += "*"
			}
			alts[stemmed] = true
		}
		if parts := WordTerms(w)[1:]; relax && len(parts) > 0 {
			alts[strings.Join(parts, " ")] = true
		}
		g := make(Group, 0, len(alts))
		for a := range alts {
			g = append(g, a)
		}
		sort.Strings(g) // byte order of UTF-8 is code-point order
		dup := false
		for _, o := range groups {
			dup = dup || o.equal(g)
		}
		if !dup {
			groups = append(groups, g)
		}
	}
	return groups
}

// queryWords are the words of query ParseQuery makes groups of: stop-words
// dropped unless nothing else is left.
func queryWords(query string) []string {
	words := Words(Fold(query))
	var keep []string
	for _, w := range words {
		if !stopWords[textfold.Lower(w)] {
			keep = append(keep, w)
		}
	}
	if len(keep) == 0 {
		keep = words
	}
	return keep
}

// Rewrites are the query words ParseQuery searches as their English
// glossary terms, each "word -> term[, term]" in query order, once per word
// ("alarmas -> alarm"). nh notes them on stderr; the ranking is ParseQuery's.
func Rewrites(query string) []string {
	var out []string
	seen := map[string]bool{}
	for _, w := range queryWords(query) {
		low := textfold.Lower(w)
		english := helpGlossary.LookupKey(low)
		if len(english) == 0 || seen[low] || len(english) == 1 && english[0] == low {
			continue
		}
		seen[low] = true
		out = append(out, low+" -> "+strings.Join(english, ", "))
	}
	return out
}

// echoLimit is how many characters of a query nh repeats in a header, an
// error or a miss block; the query itself is searched whole.
const echoLimit = 200

// Echo is query as nh repeats it: whole up to echoLimit characters, else
// cut there with the count of the characters left out.
func Echo(query string) string {
	r := []rune(query)
	if len(r) <= echoLimit {
		return query
	}
	return fmt.Sprintf("%s... (%d more characters)", string(r[:echoLimit]), len(r)-echoLimit)
}

// Matches is fulltext_terms.group_matches: a phrase needs every part, a
// prefix term any term starting with it, a plain term itself.
func (g Group) Matches(terms map[string]bool) bool {
	for _, alt := range g {
		switch {
		case strings.Contains(alt, " "):
			all := true
			for _, p := range strings.Fields(alt) {
				all = all && terms[p]
			}
			if all {
				return true
			}
		case strings.HasSuffix(alt, "*"):
			prefix := alt[:len(alt)-1]
			for t := range terms {
				if strings.HasPrefix(t, prefix) {
					return true
				}
			}
		case terms[alt]:
			return true
		}
	}
	return false
}

// MatchesWord is group_matches over set(word_terms(w)).
func (g Group) MatchesWord(w string) bool {
	set := map[string]bool{}
	for _, t := range WordTerms(w) {
		set[t] = true
	}
	return g.Matches(set)
}
