package blocksearch

import (
	"slices"
	"unicode"

	"github.com/angeles725/niagara-go/internal/bm25"
	"github.com/angeles725/niagara-go/internal/textfold"
)

// classFormWeight is the idf factor of a query word's Niagara class form: below the
// word itself, so a block naming the word ranks before one naming only its class form.
const classFormWeight = 0.5

// classForms maps each CamelCase word of the query (BacnetIpServerPort) to the ranking
// term of its Niagara class form (BBacnetIpServerPort), which the index holds as a
// different whole word. A plain word never gets one ("ox" is not "box", "Ring" not
// "Bring"), nor a word already in class form (B followed by a capital).
func classForms(query string) map[string]string {
	var out map[string]string
	for _, w := range rawWords(query) {
		if !camelCase(w) || (len(w) > 1 && w[0] == 'B' && unicode.IsUpper(rune(w[1]))) {
			continue
		}
		word, class := textfold.Terms(w), textfold.Terms("B"+w)
		if len(word) != 1 || len(class) != 1 || word[0] == class[0] {
			continue
		}
		if out == nil {
			out = map[string]string{}
		}
		out[word[0]] = class[0]
	}
	return out
}

// rawWords is the [A-Za-z0-9_] runs of the unfolded query.
func rawWords(s string) []string {
	var out []string
	start := -1
	for i := 0; i <= len(s); i++ {
		if i < len(s) && (s[i] == '_' || s[i] < 0x80 && (unicode.IsLetter(rune(s[i])) || unicode.IsDigit(rune(s[i])))) {
			if start < 0 {
				start = i
			}
			continue
		}
		if start >= 0 {
			out = append(out, s[start:i])
			start = -1
		}
	}
	return out
}

// camelCase reports whether w starts with a capital and holds another capital and a
// lower-case letter (an identifier, not a capitalised word).
func camelCase(w string) bool {
	upper, lower := 0, false
	for _, r := range w {
		upper += btoi(unicode.IsUpper(r))
		lower = lower || unicode.IsLower(r)
	}
	return unicode.IsUpper(rune(w[0])) && upper >= 2 && lower
}

func btoi(b bool) int {
	if b {
		return 1
	}
	return 0
}

// withClassForms adds each class form the query asks for (classForms) to the ranking
// terms and factors, unless the query already names it; it returns the forms added.
func withClassForms(query string, q []string, factors []bm25.Factor) ([]string, []bm25.Factor, map[string]string) {
	forms := classForms(query)
	for word, class := range forms {
		if !slices.Contains(q, word) || slices.Contains(q, class) {
			delete(forms, word)
		}
	}
	for _, word := range q { // query order, so the result does not depend on map order
		if class, ok := forms[word]; ok {
			q = append(q, class)
			factors = append(slices.Clip(factors), bm25.Factor{Term: class, Weight: classFormWeight})
		}
	}
	return q, factors, forms
}

// exactOnly drops a class form's postings in documents that hold its word, so a
// document's score from an exact hit is what it was without class forms: exact hits
// keep their order, and the form only brings in (and ranks) documents without the word.
func exactOnly(post map[string]map[int]int, df map[string]int, forms map[string]string) {
	for word, class := range forms {
		cp := post[class]
		if cp == nil || post[word] == nil {
			continue
		}
		kept := map[int]int{}
		for d, tf := range cp {
			if _, exact := post[word][d]; !exact {
				kept[d] = tf
			}
		}
		post[class], df[class] = kept, len(kept)
	}
}

// holdsEach reports whether doc d holds every query term or, for a word with a class
// form, that form (AND mode with class forms).
func holdsEach(post map[string]map[int]int, q []string, forms map[string]string, d int) bool {
	isForm := map[string]bool{}
	for _, class := range forms {
		isForm[class] = true
	}
	for _, t := range q {
		if isForm[t] {
			continue
		}
		if _, ok := post[t][d]; ok {
			continue
		}
		if _, ok := post[forms[t]][d]; !ok || forms[t] == "" {
			return false
		}
	}
	return true
}
