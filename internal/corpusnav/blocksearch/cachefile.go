package blocksearch

import (
	"bytes"
	"encoding/binary"
	"errors"
	"hash/crc32"
	"io"
	"os"
	"sort"

	"github.com/angeles725/niagara-go/internal/atomicfile"
)

// The find cache file (schema go-2). A warm find used to gob-decode every term's postings
// (~77k terms, 3.9 MB, ~50 ms) to use the few its query names; this layout is read in one
// call and only the document table is decoded: the query terms are found by binary search
// in a fixed-width term table, so a query no longer pays for the whole vocabulary.
// Little-endian throughout; str is a u32 byte length and the bytes:
//
//	magic "cnavfind" | u32 CRC-32C of the rest
//	str schema | str fingerprint | u32 ndocs | u64 total length
//	u32 n, n x str unreadable | ndocs x (str display, str label, str path, u32 length)
//	u32 nterms | nterms x (u32 term offset, u32 term length, u32 first pair, u32 pairs),
//	sorted by term | u32 n, n bytes of terms | u32 n, n x (i32 doc, i32 tf) postings
//
// Anything that does not parse exactly (magic, checksum, a length or offset out of
// bounds, bytes left over) is a corrupt cache: ranked in memory and rebuilt.

const (
	cacheMagic    = "cnavfind"
	termEntrySize = 16 // four u32
	minDocSize    = 16 // three empty strings and a length
)

var (
	castagnoli = crc32.MakeTable(crc32.Castagnoli)
	errCorrupt = errors.New("corrupt find cache")
)

// cacheFile is a parsed find cache. The term table and its blobs stay raw bytes.
type cacheFile struct {
	schema, fingerprint string
	nDocs, totalLen     int
	unreadable          []string
	docs                []Doc
	nTerms              int
	table, terms, pairs []byte
}

// reader is a bounds-checked cursor; the first short read sets err and every later
// read returns zero values.
type reader struct {
	b   []byte
	err error
}

func (r *reader) next(n int) []byte {
	if r.err != nil || n < 0 || n > len(r.b) {
		r.err = errCorrupt
		return nil
	}
	out := r.b[:n:n]
	r.b = r.b[n:]
	return out
}

func (r *reader) u32() int {
	if b := r.next(4); b != nil {
		return int(binary.LittleEndian.Uint32(b))
	}
	return 0
}

func (r *reader) u64() int {
	if b := r.next(8); b != nil {
		if v := binary.LittleEndian.Uint64(b); v <= 1<<53 {
			return int(v)
		}
		r.err = errCorrupt
	}
	return 0
}

func (r *reader) str() string { return string(r.next(r.u32())) }

// readCacheFile parses the cache at path: the os error when it cannot be opened (a
// missing cache), errCorrupt for anything after that.
func readCacheFile(path string) (*cacheFile, error) {
	fh, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer fh.Close()
	st, err := fh.Stat()
	if err != nil {
		return nil, errCorrupt
	}
	data := make([]byte, st.Size())
	if _, err := io.ReadFull(fh, data); err != nil {
		return nil, errCorrupt
	}
	return parseCache(data)
}

// parseCache checks and splits the bytes of a cache file (errCorrupt when they are not one).
func parseCache(data []byte) (*cacheFile, error) {
	if len(data) < len(cacheMagic)+4 || string(data[:len(cacheMagic)]) != cacheMagic {
		return nil, errCorrupt
	}
	body := data[len(cacheMagic)+4:]
	if crc32.Checksum(body, castagnoli) != binary.LittleEndian.Uint32(data[len(cacheMagic):]) {
		return nil, errCorrupt
	}
	r := &reader{b: body}
	f := &cacheFile{schema: r.str(), fingerprint: r.str(), nDocs: r.u32(), totalLen: r.u64()}
	nUnreadable := r.u32()
	if nUnreadable > len(r.b)/4 || f.nDocs > len(r.b)/minDocSize {
		return nil, errCorrupt // the counts cannot fit: do not allocate for them
	}
	f.unreadable = make([]string, 0, nUnreadable)
	for ; nUnreadable > 0 && r.err == nil; nUnreadable-- {
		f.unreadable = append(f.unreadable, r.str())
	}
	f.docs = make([]Doc, 0, f.nDocs)
	for n := f.nDocs; n > 0 && r.err == nil; n-- {
		f.docs = append(f.docs, Doc{Display: r.str(), Label: r.str(), Path: r.str(), Length: r.u32()})
	}
	f.nTerms = r.u32()
	f.table = r.next(f.nTerms * termEntrySize)
	f.terms = r.next(r.u32())
	f.pairs = r.next(r.u32() * 8)
	if r.err != nil || len(r.b) != 0 || len(f.docs) != f.nDocs {
		return nil, errCorrupt
	}
	return f, nil
}

// postings is the (doc, tf) pairs of term, nil when the term is not indexed.
func (f *cacheFile) postings(term string) (map[int]int, error) {
	entry := func(i int) (off, n, first, pairs int) {
		e := f.table[i*termEntrySize:]
		return int(binary.LittleEndian.Uint32(e)), int(binary.LittleEndian.Uint32(e[4:])),
			int(binary.LittleEndian.Uint32(e[8:])), int(binary.LittleEndian.Uint32(e[12:]))
	}
	text := func(i int) []byte {
		off, n, _, _ := entry(i)
		if off > len(f.terms) || n > len(f.terms)-off {
			return nil
		}
		return f.terms[off : off+n]
	}
	key := []byte(term)
	bad := false
	i := sort.Search(f.nTerms, func(i int) bool {
		t := text(i)
		bad = bad || t == nil
		return bytes.Compare(t, key) >= 0
	})
	if bad {
		return nil, errCorrupt
	}
	if i == f.nTerms || !bytes.Equal(text(i), key) {
		return nil, nil
	}
	_, _, first, n := entry(i)
	if first > len(f.pairs)/8 || n > len(f.pairs)/8-first {
		return nil, errCorrupt
	}
	post := make(map[int]int, n)
	for p := f.pairs[first*8 : (first+n)*8]; len(p) > 0; p = p[8:] {
		doc := int(int32(binary.LittleEndian.Uint32(p)))
		if doc < 0 || doc >= len(f.docs) {
			return nil, errors.New("postings point outside the document table")
		}
		post[doc] = int(int32(binary.LittleEndian.Uint32(p[4:])))
	}
	return post, nil
}

// writeCacheFile writes mem with fingerprint fp to path atomically
// (atomicfile.WriteStream: a temp file in the same directory, then rename),
// with mode 0600.
func writeCacheFile(mem *MemIndex, fp, path string) error {
	data := encodeCache(mem, fp)
	return atomicfile.WriteStream(path, 0o600, func(w io.Writer) error {
		_, err := w.Write(data)
		return err
	})
}

// encodeCache is the cache file bytes of mem: each term's pairs in document order.
func encodeCache(mem *MemIndex, fp string) []byte {
	post := map[string][]int32{}
	total := 0
	for i, tf := range mem.tfs {
		total += mem.docs[i].Length
		for t, n := range tf {
			post[t] = append(post[t], int32(i), int32(n))
		}
	}
	terms := make([]string, 0, len(post))
	for t := range post {
		terms = append(terms, t)
	}
	sort.Strings(terms)

	le := binary.LittleEndian
	b := []byte(cacheMagic + "\x00\x00\x00\x00")
	str := func(s string) { b = append(le.AppendUint32(b, uint32(len(s))), s...) }
	str(indexSchema)
	str(fp)
	b = le.AppendUint32(b, uint32(len(mem.docs)))
	b = le.AppendUint64(b, uint64(total))
	b = le.AppendUint32(b, uint32(len(mem.unreadable)))
	for _, p := range mem.unreadable {
		str(p)
	}
	for _, d := range mem.docs {
		str(d.Display)
		str(d.Label)
		str(d.Path)
		b = le.AppendUint32(b, uint32(d.Length))
	}
	b = le.AppendUint32(b, uint32(len(terms)))
	var blob []byte
	var pairs []int32
	for _, t := range terms {
		b = le.AppendUint32(b, uint32(len(blob)))
		b = le.AppendUint32(b, uint32(len(t)))
		b = le.AppendUint32(b, uint32(len(pairs)/2))
		b = le.AppendUint32(b, uint32(len(post[t])/2))
		blob = append(blob, t...)
		pairs = append(pairs, post[t]...)
	}
	b = append(le.AppendUint32(b, uint32(len(blob))), blob...)
	b = le.AppendUint32(b, uint32(len(pairs)/2))
	for _, v := range pairs {
		b = le.AppendUint32(b, uint32(v))
	}
	le.PutUint32(b[len(cacheMagic):], crc32.Checksum(b[len(cacheMagic)+4:], castagnoli))
	return b
}
