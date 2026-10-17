package textfold

// The ranking terms of tools/lib/textsearch.py (terms, query_terms, stem):
// folded with Corpus, tokenized on [a-z0-9_]+, stop-words and 1-character
// tokens dropped, then the light plural stemmer. module-navigator and
// niagara-help analyse text their own way (docs/text-variants.md).

// textsearchStopwords is textsearch.STOPWORDS (102 English and Spanish words).
var textsearchStopwords = setOf(`
a an and are as at be by can could do does for from has have how i if in into is it its
me my of on or our should that the their then there these this those to was we were what
when where which who why will with without you your
al como con cual cuales cuando de del donde el en entre es esta este esto estos estas
hay la las le les lo los mas muy no o para pero por porque que se si sin sobre son su sus
un una unas uno unos y ya
`)

func setOf(words string) map[string]bool {
	out := map[string]bool{}
	for _, w := range Fields(words) {
		out[w] = true
	}
	return out
}

// IsStopword reports textsearch.STOPWORDS membership.
func IsStopword(token string) bool { return textsearchStopwords[token] }

// Stem is textsearch.stem: drop one final "s" from a token longer than four
// characters that does not end in "ss" ("alarms" -> "alarm", "class" kept).
func Stem(token string) string {
	if len([]rune(token)) > 4 && token[len(token)-1] == 's' && token[len(token)-2] != 's' {
		return token[:len(token)-1]
	}
	return token
}

// Normalize is textsearch._normalize: the ranking term of one folded
// [a-z0-9_] token, "" for a stop-word or a 1-character token.
func Normalize(token string) string {
	if len(token) <= 1 || textsearchStopwords[token] {
		return ""
	}
	return Stem(token)
}

// Tokens returns the [a-z0-9_]+ runs of an already folded text, in order.
func Tokens(folded string) []string {
	var out []string
	start := -1
	for i := 0; i <= len(folded); i++ {
		if i < len(folded) && isTermByte(folded[i]) {
			if start < 0 {
				start = i
			}
			continue
		}
		if start >= 0 {
			out = append(out, folded[start:i])
			start = -1
		}
	}
	return out
}

func isTermByte(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '_'
}

// Terms is textsearch.terms: the ranking terms of text, in order, repeats kept.
func Terms(text string) []string {
	var out []string
	for _, tok := range Tokens(Fold(text, Corpus)) {
		if t := Normalize(tok); t != "" {
			out = append(out, t)
		}
	}
	return out
}

// QueryTerms is textsearch.query_terms: the distinct ranking terms of a
// query, in query order.
func QueryTerms(query string) []string {
	return Unique(Terms(query))
}

// Unique keeps the first occurrence of each string, in order
// (Python's list(dict.fromkeys(xs))).
func Unique(xs []string) []string {
	seen := make(map[string]bool, len(xs))
	out := make([]string, 0, len(xs))
	for _, x := range xs {
		if !seen[x] {
			seen[x] = true
			out = append(out, x)
		}
	}
	return out
}
