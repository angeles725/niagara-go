package glossary

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"

	"github.com/angeles725/niagara-go/internal/bm25"
	"github.com/angeles725/niagara-go/internal/textfold"
)

//go:embed glossary_es_en.json
var embedded []byte

// TranslationWeight is glossary.TRANSLATION_WEIGHT: the idf factor of an
// English translation next to the user's own words (corpus-nav find).
const TranslationWeight = 0.5

// HelpExcluded is niagara-help search_lang.EXCLUDED: canonical words its
// full-text search leaves untranslated.
var HelpExcluded = []string{"guardan", "montar"}

// Glossary maps Spanish keys to English terms, keeping the file's key order.
type Glossary struct {
	keys  []string
	terms map[string][]string
}

var canonical = sync.OnceValue(func() *Glossary {
	g, bad, err := Parse(embedded)
	if err != nil || len(bad) > 0 {
		panic(fmt.Sprintf("glossary: embedded glossary_es_en.json is broken (bad %v): %v", bad, err))
	}
	return g
})

// Default is the canonical glossary (tools/lib/glossary.py glossary()).
func Default() *Glossary { return canonical() }

// Help is niagara-help search_lang.GLOSSARY: Default without HelpExcluded.
func Help() *Glossary { return Default().Without(HelpExcluded...) }

// ClassRankByStem is module-navigator class_rank._GLOSSARY_BY_STEM: the
// canonical entries keyed by textfold.Stem of the key, since its query terms
// reach the lookup already stemmed.
func ClassRankByStem() map[string][]string { return Default().KeyedBy(textfold.Stem) }

// Parse reads glossary JSON the way glossary.py does: the object under
// "glossary"; a string value is one term, a list of strings is the terms, and
// any other value is skipped and its key returned in bad (file order). A
// duplicated key keeps its first position and its last value, like json.load.
func Parse(data []byte) (g *Glossary, bad []string, err error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	top, err := objectMembers(dec)
	if err != nil {
		return nil, nil, err
	}
	if _, err := dec.Token(); err != io.EOF {
		return nil, nil, errors.New("glossary: extra data after the JSON object")
	}
	raw, ok := top.get("glossary")
	if !ok {
		return nil, nil, errors.New(`glossary: no "glossary" object`)
	}
	members, err := objectMembers(json.NewDecoder(bytes.NewReader(raw)))
	if err != nil {
		return nil, nil, fmt.Errorf(`glossary: "glossary" is not an object: %w`, err)
	}
	g = &Glossary{terms: map[string][]string{}}
	for i, key := range members.keys {
		terms, ok := termsOf(members.values[i])
		if !ok {
			bad = append(bad, key)
			continue
		}
		g.keys = append(g.keys, key)
		g.terms[key] = terms
	}
	return g, bad, nil
}

type orderedObject struct {
	keys   []string
	values []json.RawMessage
}

func (o orderedObject) get(key string) (json.RawMessage, bool) {
	for i, k := range o.keys {
		if k == key {
			return o.values[i], true
		}
	}
	return nil, false
}

// objectMembers reads one JSON object, members in first-seen order, last value wins.
func objectMembers(dec *json.Decoder) (orderedObject, error) {
	var o orderedObject
	tok, err := dec.Token()
	if err != nil {
		return o, err
	}
	if d, ok := tok.(json.Delim); !ok || d != '{' {
		return o, errors.New("not a JSON object")
	}
	index := map[string]int{}
	for dec.More() {
		kt, err := dec.Token()
		if err != nil {
			return o, err
		}
		key := kt.(string)
		var v json.RawMessage
		if err := dec.Decode(&v); err != nil {
			return o, err
		}
		if i, seen := index[key]; seen {
			o.values[i] = v
			continue
		}
		index[key] = len(o.keys)
		o.keys = append(o.keys, key)
		o.values = append(o.values, v)
	}
	_, err = dec.Token() // the closing brace
	return o, err
}

func termsOf(raw json.RawMessage) ([]string, bool) {
	var v any
	if json.Unmarshal(raw, &v) != nil {
		return nil, false
	}
	switch x := v.(type) {
	case string:
		return []string{x}, true
	case []any:
		out := make([]string, 0, len(x))
		for _, item := range x {
			s, ok := item.(string)
			if !ok {
				return nil, false
			}
			out = append(out, s)
		}
		return out, true
	}
	return nil, false
}

// Without is g minus the given keys.
func (g *Glossary) Without(keys ...string) *Glossary {
	drop := map[string]bool{}
	for _, k := range keys {
		drop[k] = true
	}
	out := &Glossary{terms: map[string][]string{}}
	for _, k := range g.keys {
		if !drop[k] {
			out.keys = append(out.keys, k)
			out.terms[k] = g.terms[k]
		}
	}
	return out
}

// KeyedBy re-keys the entries by f(key); a later key wins a shared f(key).
func (g *Glossary) KeyedBy(f func(string) string) map[string][]string {
	out := make(map[string][]string, len(g.keys))
	for _, k := range g.keys {
		out[f(k)] = g.terms[k]
	}
	return out
}

// LookupKey is the singular-key search of glossary.lookup and niagara-help's
// fulltext_terms._glossary on an already folded, lower-case word: the word,
// then without a final "s", then without a final "es". nil when none is a key.
func (g *Glossary) LookupKey(word string) []string {
	cands := []string{word}
	if strings.HasSuffix(word, "s") {
		cands = append(cands, word[:len(word)-1])
	}
	if strings.HasSuffix(word, "es") {
		cands = append(cands, word[:len(word)-2])
	}
	for _, c := range cands {
		if t, ok := g.terms[c]; c != "" && ok {
			return t
		}
	}
	return nil
}

// Lookup is glossary.lookup: the English terms of one Spanish word in any
// case, accents or plural (folded with textfold.Corpus first).
func (g *Glossary) Lookup(word string) []string {
	return g.LookupKey(textfold.Fold(word, textfold.Corpus))
}

// Translation is one glossary word of a query and its English terms.
type Translation struct {
	Word  string
	Terms []string
}

// Translations is glossary.translations: the glossary words of query, in
// query order, once each.
func (g *Glossary) Translations(query string) []Translation {
	var out []Translation
	seen := map[string]bool{}
	for _, word := range textfold.Tokens(textfold.Fold(query, textfold.Corpus)) {
		if terms := g.Lookup(word); len(terms) > 0 && !seen[word] {
			seen[word] = true
			out = append(out, Translation{word, terms})
		}
	}
	return out
}

// Expand is glossary.expand: query plus the English terms of its Spanish
// words that it does not already contain.
func (g *Glossary) Expand(query string) string {
	have := map[string]bool{}
	for _, w := range textfold.Tokens(textfold.Fold(query, textfold.Corpus)) {
		have[w] = true
	}
	var extra []string
	for _, tr := range g.Translations(query) {
		for _, t := range tr.Terms {
			if !have[t] {
				extra = append(extra, t)
			}
		}
	}
	extra = textfold.Unique(extra)
	if len(extra) == 0 {
		return query
	}
	return query + " " + strings.Join(extra, " ")
}

// WeightedTerms is glossary.weighted_terms: the ranking terms of the English
// translations that are not ranking terms of the query, each at
// TranslationWeight, in first-seen order (bm25 factors).
func (g *Glossary) WeightedTerms(query string) []bm25.Factor {
	own := map[string]bool{}
	for _, t := range textfold.Terms(query) {
		own[t] = true
	}
	var out []bm25.Factor
	seen := map[string]bool{}
	for _, tr := range g.Translations(query) {
		for _, t := range textfold.Terms(strings.Join(tr.Terms, " ")) {
			if !own[t] && !seen[t] {
				seen[t] = true
				out = append(out, bm25.Factor{Term: t, Weight: TranslationWeight})
			}
		}
	}
	return out
}

// Note is glossary.note: "Spanish terms: alarma→alarm, ..." or "".
func (g *Glossary) Note(query string) string {
	tr := g.Translations(query)
	if len(tr) == 0 {
		return ""
	}
	parts := make([]string, len(tr))
	for i, t := range tr {
		parts[i] = t.Word + "→" + strings.Join(t.Terms, "/")
	}
	return "Spanish terms: " + strings.Join(parts, ", ")
}
