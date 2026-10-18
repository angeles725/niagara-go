package blocksearch

import (
	"crypto/sha256"
	"encoding/gob"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/angeles725/niagara-go/internal/bm25"
	"github.com/angeles725/niagara-go/internal/pycompat"
	"github.com/angeles725/niagara-go/internal/textfold"
)

// corpusnav/findindex.py: BM25 (textsearch, title counted 8x) over a search scope, in
// memory or from a cache file. The cache is a gob file, find-<scope>.gob: a private
// gob cache avoids a SQLite dependency for a small index (the Python tool keeps
// find-<scope>.sqlite; the two never read each other's file). It holds the
// document table and the (doc, tf) postings of every term; it is trusted only when its
// fingerprint (schema, this executable, the stat of CATALOG.md and of every searched
// file) matches, else the query is ranked in memory and the cache rebuilt atomically.

const indexSchema = "go-1"

// Doc is one searchable file. text is loaded on demand for a cached document.
type Doc struct {
	Display, Label, Path string
	Length               int
	text                 string
	hasText              bool
}

// Text is Target.text: the file's text, read now for a document from the cache.
func (d *Doc) Text() (string, bool) {
	if !d.hasText {
		d.text, d.hasText = ReadText(d.Path)
		if !d.hasText {
			return "", false
		}
	}
	return d.text, true
}

// stored is the cache file content (exported fields for gob).
type stored struct {
	Schema, Fingerprint string
	NDocs, TotalLen     int
	Unreadable          []string
	Docs                []Doc
	Terms               map[string][]int32 // term -> doc, tf, doc, tf, ...
}

// MemIndex is findindex.SearchIndex: every readable target of a scope, tokenized.
type MemIndex struct {
	nFiles     int
	unreadable []string
	docs       []Doc
	tfs        []map[string]int
}

// NewMemIndex reads and tokenizes every target (title counted 8x).
func (c *Corpus) NewMemIndex(targets []Target) *MemIndex {
	catalog := c.LoadCatalog()
	ix := &MemIndex{nFiles: len(targets)}
	for _, t := range targets {
		text, ok := ReadText(t.Path)
		if !ok {
			ix.unreadable = append(ix.unreadable, t.Path)
			continue
		}
		label := MainLabel(t.Display)
		h1 := ""
		for _, ln := range pycompat.SplitLines(text, false) {
			if strings.HasPrefix(ln, "# ") {
				h1 = pycompat.Strip(ln[2:])
				break
			}
		}
		title := h1 // the CATALOG title can be a curated Spanish gloss of an English H1: index both
		if label != "" {
			title = catalog[label] + " " + h1
		}
		tf := map[string]int{}
		for _, part := range []struct {
			weight int
			text   string
		}{{1, text}, {8, title}} {
			for _, tok := range textfold.Tokens(fold(part.text)) {
				if term := textfold.Normalize(tok); term != "" {
					tf[term] += part.weight
				}
			}
		}
		n := 0
		for _, v := range tf {
			n += v
		}
		ix.docs = append(ix.docs, Doc{Display: t.Display, Label: label, Path: t.Path, Length: n, text: text, hasText: true})
		ix.tfs = append(ix.tfs, tf)
	}
	return ix
}

// Scored is one ranked document.
type Scored struct {
	Score float64
	Doc   *Doc
}

// queryTerms is the query's ranking terms plus the factored terms not among them.
func queryTerms(query string, factors []bm25.Factor) []string {
	q := textfold.QueryTerms(query)
	for _, f := range factors {
		dup := false
		for _, t := range q {
			dup = dup || t == f.Term
		}
		if !dup {
			q = append(q, f.Term)
		}
	}
	return q
}

func rankStats(docs []Doc, post map[string]map[int]int, df map[string]int, total int, q []string,
	requireAll bool, factors []bm25.Factor) []Scored {
	st := bm25.Stats{NDocs: len(docs), DF: df, Post: post, Len: make([]int, len(docs))}
	for i, d := range docs {
		st.Len[i] = d.Length
	}
	if len(docs) > 0 {
		st.AvgLen = float64(total) / float64(len(docs))
	}
	var out []Scored
	for _, s := range bm25.Textsearch.Rank(q, st, requireAll, factors) {
		out = append(out, Scored{s.Score, &docs[s.Doc]})
	}
	return out
}

// Rank is SearchIndex.rank: BM25 over the in-memory index.
func (ix *MemIndex) Rank(query string, requireAll bool, factors []bm25.Factor) []Scored {
	q := queryTerms(query, factors)
	df, post, total := map[string]int{}, map[string]map[int]int{}, 0
	for i, tf := range ix.tfs {
		total += ix.docs[i].Length
		for _, t := range q {
			if n, ok := tf[t]; ok {
				df[t]++
				if post[t] == nil {
					post[t] = map[int]int{}
				}
				post[t][i] = n
			}
		}
	}
	return rankStats(ix.docs, post, df, total, q, requireAll, factors)
}

func (s *stored) rank(query string, requireAll bool, factors []bm25.Factor) ([]Scored, error) {
	q := queryTerms(query, factors)
	df, post := map[string]int{}, map[string]map[int]int{}
	for _, t := range q {
		pairs, ok := s.Terms[t]
		if !ok {
			continue
		}
		if len(pairs)%2 != 0 {
			return nil, fmt.Errorf("postings of %q: odd length", t)
		}
		post[t] = map[int]int{}
		for i := 0; i < len(pairs); i += 2 {
			if int(pairs[i]) < 0 || int(pairs[i]) >= len(s.Docs) {
				return nil, errors.New("postings point outside the document table")
			}
			post[t][int(pairs[i])] = int(pairs[i+1])
		}
		df[t] = len(pairs) / 2
	}
	return rankStats(s.Docs, post, df, s.TotalLen, q, requireAll, factors), nil
}

// Ranking is findindex.ranked's answer.
type Ranking struct {
	Results    []Scored
	NFiles     int
	Unreadable []string
	Fallback   string // "", or why the cache was not used: missing, stale, corrupt
	Note       string // the stderr line corpus-nav prints when it ranked in memory, or ""
}

// IndexPath is the find cache of a scope: find-<scope><CacheTag>.gob in IndexDir.
func (c *Corpus) IndexPath(scope string) string {
	return PyJoin(c.IndexDir, "find-"+scope+c.CacheTag+".gob")
}

// fingerprint hashes the schema, this executable and the stat of CATALOG.md and of every
// searched file.
func (c *Corpus) fingerprint(targets []Target) string {
	h := sha256.New()
	h.Write([]byte(indexSchema))
	if exe, err := os.Executable(); err == nil {
		fmt.Fprintf(h, "\x00%s", statStamp(exe))
	}
	for _, t := range append([]Target{{"CATALOG", c.Catalog}}, targets...) {
		fmt.Fprintf(h, "%s\x00%s\x00%s\n", t.Display, t.Path, statStamp(t.Path))
	}
	return hex.EncodeToString(h.Sum(nil))
}

func indexDisabled() bool {
	switch strings.ToLower(pycompat.Strip(os.Getenv("CORPUS_NAV_FIND_INDEX"))) {
	case "off", "0", "no", "false":
		return true
	}
	return false
}

// Ranked is findindex.ranked: from the cache when fresh, else in memory (rebuilding the
// cache; Note is the line corpus-nav prints on stderr then).
func (c *Corpus) Ranked(scope, query string, requireAll bool, factors []bm25.Factor) Ranking {
	targets := c.Targets(scope)
	if indexDisabled() {
		mem := c.NewMemIndex(targets)
		return Ranking{Results: mem.Rank(query, requireAll, factors), NFiles: len(targets), Unreadable: mem.unreadable}
	}
	path := c.IndexPath(scope)
	fp := c.fingerprint(targets) // stat BEFORE reading: a file changed meanwhile leaves the cache stale
	state := "missing"
	if f, err := os.Open(path); err == nil {
		var s stored
		derr := gob.NewDecoder(f).Decode(&s)
		f.Close()
		state = "corrupt"
		if derr == nil && len(s.Docs) == s.NDocs {
			state = "stale"
			if s.Schema == indexSchema && s.Fingerprint == fp {
				if res, err := s.rank(query, requireAll, factors); err == nil {
					return Ranking{Results: res, NFiles: len(targets), Unreadable: s.Unreadable}
				}
				state = "corrupt"
			}
		}
	}
	mem := c.NewMemIndex(targets)
	outcome := "index rebuilt"
	if err := writeIndex(mem, fp, path); err != nil {
		outcome = "index not rebuilt: " + strings.SplitN(err.Error(), "\n", 2)[0]
	}
	note := fmt.Sprintf("corpus-nav: find index %s (%s); ranked in memory, %s\n", state, path, outcome)
	return Ranking{mem.Rank(query, requireAll, factors), len(targets), mem.unreadable, state, note}
}

// writeIndex writes the cache atomically: a temp file in the same directory, then rename.
func writeIndex(mem *MemIndex, fp, path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o777); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".*.tmp")
	if err != nil {
		return err
	}
	s := stored{Schema: indexSchema, Fingerprint: fp, NDocs: len(mem.docs), Unreadable: mem.unreadable,
		Docs: mem.docs, Terms: map[string][]int32{}}
	for i, tf := range mem.tfs {
		s.TotalLen += mem.docs[i].Length
		for t, n := range tf {
			s.Terms[t] = append(s.Terms[t], int32(i), int32(n))
		}
	}
	if s.Unreadable == nil {
		s.Unreadable = []string{}
	}
	err = gob.NewEncoder(tmp).Encode(&s)
	if cerr := tmp.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		err = os.Rename(tmp.Name(), path)
	}
	if err != nil {
		os.Remove(tmp.Name())
	}
	return err
}
