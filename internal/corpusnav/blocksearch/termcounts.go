package blocksearch

import (
	"encoding/binary"
	"errors"
	"hash/crc32"
	"io"
	"os"
	"slices"
	"sort"

	"github.com/angeles725/niagara-go/internal/atomicfile"
	"github.com/angeles725/niagara-go/internal/pypath"
)

// Per-file term counts under a caller's tokenizer (corpus-nav semantic's TF-IDF), counted
// in memory or read from a cache file beside the find cache, <name><CacheTag>.gob in
// IndexDir. It is the find cache's mechanism, not another one: the same fingerprint
// (schema, this executable, the stat of CATALOG.md and of every file), the same
// CRC-sealed little-endian layout read in one call, the same atomic 0600 write and the
// same CORPUS_NAV_FIND_INDEX=off switch. The tokenizer is this executable's code, so
// the executable's stamp in the fingerprint covers a change to it. Layout:
//
//	magic "cnavterm" | u32 CRC-32C of the rest
//	str schema | str name | str fingerprint | u32 ndocs | ndocs x (u8 readable, u32 terms)
//	u32 nterms | str vocabulary bytes | nterms x u32 term length (terms sorted, distinct)
//	sum(terms) x (u32 term id, u32 count), each document's terms in first-occurrence order
//
// Anything that does not parse exactly is a corrupt cache: counted again and rewritten.

const (
	termMagic  = "cnavterm"
	termSchema = "terms-1"
)

var errTermsStale = errors.New("term-count cache for another name or fingerprint")

// TermCounts is how often each term occurs in each file of a list.
type TermCounts struct {
	Vocab []string // every term, sorted, distinct
	Docs  []TermDoc
}

// TermDoc is one file: its terms (ids into Vocab) in first-occurrence order and their
// counts. A file that cannot be read has no terms.
type TermDoc struct {
	Readable bool
	IDs      []int32
	Counts   []int32
}

// ID is the vocabulary id of term, -1 when no file holds it.
func (tc *TermCounts) ID(term string) int {
	if i, ok := slices.BinarySearch(tc.Vocab, term); ok {
		return i
	}
	return -1
}

// CountTerms reads and tokenizes every path. Every document's ids and counts share
// two backing arrays, and no per-document map is built.
func CountTerms(paths []string, tokenize func(string) []string) *TermCounts {
	ids := map[string]int32{}
	var vocab []string
	var lastDoc, slot []int32 // per term: the last document that held it, its index there
	var allIDs, allCounts []int32
	readable := make([]bool, len(paths))
	ends := make([]int, len(paths))
	for doc, p := range paths {
		text, ok := ReadText(p)
		readable[doc] = ok
		start := len(allIDs)
		for _, t := range tokenize(text) {
			id, known := ids[t]
			if !known {
				id = int32(len(vocab))
				ids[t] = id
				vocab = append(vocab, t)
				lastDoc = append(lastDoc, -1)
				slot = append(slot, 0)
			}
			if lastDoc[id] != int32(doc) {
				lastDoc[id], slot[id] = int32(doc), int32(len(allIDs)-start)
				allIDs = append(allIDs, id)
				allCounts = append(allCounts, 0)
			}
			allCounts[start+int(slot[id])]++
		}
		ends[doc] = len(allIDs)
	}
	// renumber the ids in vocabulary order
	order := make([]int32, len(vocab))
	for i := range order {
		order[i] = int32(i)
	}
	sort.Slice(order, func(a, b int) bool { return vocab[order[a]] < vocab[order[b]] })
	rank := make([]int32, len(vocab))
	sorted := make([]string, len(vocab))
	for r, id := range order {
		rank[id], sorted[r] = int32(r), vocab[id]
	}
	for i, id := range allIDs {
		allIDs[i] = rank[id]
	}
	return split(sorted, readable, ends, allIDs, allCounts)
}

// split cuts the shared id and count arrays into documents ending at ends.
func split(vocab []string, readable []bool, ends []int, ids, counts []int32) *TermCounts {
	tc := &TermCounts{Vocab: vocab, Docs: make([]TermDoc, len(readable))}
	start := 0
	for i, end := range ends {
		tc.Docs[i] = TermDoc{readable[i], ids[start:end:end], counts[start:end:end]}
		start = end
	}
	return tc
}

// TermCountsPath is the term-count cache called name: <name><CacheTag>.gob in IndexDir.
func (c *Corpus) TermCountsPath(name string) string {
	return pypath.Join(c.IndexDir, name+c.CacheTag+".gob")
}

// CachedTermCounts is CountTerms over targets through the cache called name, and how it
// went: "hit", or why it counted: "missing", "stale", "corrupt", "off". A cache that
// cannot be written is left as it is (the counts are the same either way).
func (c *Corpus) CachedTermCounts(name string, targets []Target, tokenize func(string) []string) (*TermCounts, string) {
	paths := make([]string, len(targets))
	for i, t := range targets {
		paths[i] = t.Path
	}
	if indexDisabled() {
		return CountTerms(paths, tokenize), "off"
	}
	path := c.TermCountsPath(name)
	fp := c.fingerprint(targets) // stat BEFORE reading: a file changed meanwhile leaves the cache stale
	state := "missing"
	if data, err := os.ReadFile(path); err == nil {
		tc, err := parseTermCounts(data, name, fp)
		if err == nil && len(tc.Docs) == len(targets) {
			return tc, "hit"
		}
		state = "corrupt"
		if errors.Is(err, errTermsStale) {
			state = "stale"
		}
	}
	tc := CountTerms(paths, tokenize)
	data := encodeTermCounts(tc, name, fp)
	_ = atomicfile.WriteStream(path, 0o600, func(w io.Writer) error {
		_, err := w.Write(data)
		return err
	})
	return tc, state
}

// encodeTermCounts is the cache file bytes of tc.
func encodeTermCounts(tc *TermCounts, name, fp string) []byte {
	le := binary.LittleEndian
	b := []byte(termMagic + "\x00\x00\x00\x00")
	str := func(s string) { b = append(le.AppendUint32(b, uint32(len(s))), s...) }
	str(termSchema)
	str(name)
	str(fp)
	b = le.AppendUint32(b, uint32(len(tc.Docs)))
	pairs := 0
	for _, d := range tc.Docs {
		flag := byte(0)
		if d.Readable {
			flag = 1
		}
		b = le.AppendUint32(append(b, flag), uint32(len(d.IDs)))
		pairs += len(d.IDs)
	}
	b = le.AppendUint32(b, uint32(len(tc.Vocab)))
	blob := 0
	for _, t := range tc.Vocab {
		blob += len(t)
	}
	b = le.AppendUint32(b, uint32(blob))
	for _, t := range tc.Vocab {
		b = append(b, t...)
	}
	for _, t := range tc.Vocab {
		b = le.AppendUint32(b, uint32(len(t)))
	}
	b = slices.Grow(b, pairs*8)
	for _, d := range tc.Docs {
		for k, id := range d.IDs {
			b = le.AppendUint32(le.AppendUint32(b, uint32(id)), uint32(d.Counts[k]))
		}
	}
	le.PutUint32(b[len(termMagic):], crc32.Checksum(b[len(termMagic)+4:], castagnoli))
	return b
}

// parseTermCounts checks and decodes a cache file: errTermsStale when it is sound but
// for another schema, name or fingerprint, errCorrupt for anything else wrong.
func parseTermCounts(data []byte, name, fp string) (*TermCounts, error) {
	if len(data) < len(termMagic)+4 || string(data[:len(termMagic)]) != termMagic {
		return nil, errCorrupt
	}
	body := data[len(termMagic)+4:]
	if crc32.Checksum(body, castagnoli) != binary.LittleEndian.Uint32(data[len(termMagic):]) {
		return nil, errCorrupt
	}
	r := &reader{b: body}
	schema, gotName, gotFP := r.str(), r.str(), r.str()
	if r.err != nil {
		return nil, errCorrupt
	}
	if schema != termSchema || gotName != name || gotFP != fp {
		return nil, errTermsStale
	}
	nDocs := r.u32()
	if nDocs > len(r.b)/5 {
		return nil, errCorrupt // the count cannot fit: do not allocate for it
	}
	readable := make([]bool, nDocs)
	ends := make([]int, nDocs)
	pairs := 0
	for i := range nDocs {
		flag := r.next(1)
		n := r.u32()
		if r.err != nil || flag[0] > 1 || n > len(r.b)/8 {
			return nil, errCorrupt
		}
		readable[i] = flag[0] == 1
		pairs += n
		ends[i] = pairs
	}
	nTerms := r.u32()
	blob := string(r.next(r.u32()))
	lengths := r.next(nTerms * 4)
	raw := r.next(pairs * 8)
	if r.err != nil || len(r.b) != 0 {
		return nil, errCorrupt
	}
	vocab := make([]string, nTerms)
	off := 0
	for i := range vocab {
		n := int(binary.LittleEndian.Uint32(lengths[i*4:]))
		if n > len(blob)-off {
			return nil, errCorrupt
		}
		vocab[i] = blob[off : off+n]
		off += n
		if i > 0 && vocab[i-1] >= vocab[i] {
			return nil, errCorrupt // ID's binary search needs sorted, distinct terms
		}
	}
	if off != len(blob) {
		return nil, errCorrupt
	}
	ids, counts := make([]int32, pairs), make([]int32, pairs)
	for i := range pairs {
		id, n := binary.LittleEndian.Uint32(raw[i*8:]), binary.LittleEndian.Uint32(raw[i*8+4:])
		if id >= uint32(nTerms) || n == 0 || n > 1<<31-1 {
			return nil, errCorrupt
		}
		ids[i], counts[i] = int32(id), int32(n)
	}
	return split(vocab, readable, ends, ids, counts), nil
}
