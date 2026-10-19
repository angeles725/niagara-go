package blocksearch

import (
	"bytes"
	"encoding/binary"
	"encoding/gob"
	"fmt"
	"hash/crc32"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func fixtureIndex(t *testing.T) (*Corpus, *MemIndex) {
	t.Helper()
	c := New(filepath.Join("..", "testdata", "corpus"), t.TempDir())
	return c, c.NewMemIndex(c.Targets("all"))
}

// TestCacheFileRoundTrip: the file read back holds the documents, the unreadable list and,
// for every term, exactly the in-memory (doc, tf) postings; an unknown term has none.
func TestCacheFileRoundTrip(t *testing.T) {
	_, mem := fixtureIndex(t)
	mem.unreadable = []string{"docs/a.md", "docs/b.md"}
	path := filepath.Join(t.TempDir(), "find-all.gob")
	if err := writeCacheFile(mem, "fp", path); err != nil {
		t.Fatal(err)
	}
	f, err := readCacheFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if f.schema != indexSchema || f.fingerprint != "fp" || !reflect.DeepEqual(f.unreadable, mem.unreadable) {
		t.Errorf("header: %q %q %q", f.schema, f.fingerprint, f.unreadable)
	}
	total := 0
	for _, d := range mem.docs {
		total += d.Length
	}
	if len(f.docs) != len(mem.docs) || f.totalLen != total {
		t.Fatalf("%d docs, total %d; want %d, %d", len(f.docs), f.totalLen, len(mem.docs), total)
	}
	for i, d := range mem.docs {
		g := f.docs[i]
		if g.Display != d.Display || g.Label != d.Label || g.Path != d.Path || g.Length != d.Length {
			t.Errorf("doc %d: %+v, want %+v", i, g, d)
		}
	}
	want := map[string]map[int]int{}
	for i, tf := range mem.tfs {
		for term, n := range tf {
			if want[term] == nil {
				want[term] = map[int]int{}
			}
			want[term][i] = n
		}
	}
	if f.nTerms != len(want) || len(want) < 50 {
		t.Fatalf("%d terms, want %d (and a fixture of at least 50)", f.nTerms, len(want))
	}
	for term, w := range want {
		if got, err := f.postings(term); err != nil || !reflect.DeepEqual(got, w) {
			t.Fatalf("postings(%q) = %v, %v; want %v", term, got, err, w)
		}
	}
	for _, term := range []string{"", "zzzzzz", "\xff"} {
		if got, err := f.postings(term); got != nil || err != nil {
			t.Errorf("postings(%q) = %v, %v; want none", term, got, err)
		}
	}
}

// reseal rewrites the checksum of a cache file edited in place.
func reseal(b []byte) []byte {
	binary.LittleEndian.PutUint32(b[len(cacheMagic):], crc32.Checksum(b[len(cacheMagic)+4:], castagnoli))
	return b
}

// TestCacheFileCorrupt: what is not exactly a cache file of this layout reads as corrupt
// (never as a cache with wrong postings) and Ranked then says so and rebuilds; bad offsets
// inside a well-formed file are caught when a query reads them; another schema is stale.
func TestCacheFileCorrupt(t *testing.T) {
	t.Setenv("CORPUS_NAV_FIND_INDEX", "")
	c, _ := fixtureIndex(t)
	mem := c.NewMemIndex(c.Targets("docs"))
	good := encodeCache(mem, c.fingerprint(c.Targets("docs")))
	parsed, err := parseCache(good)
	if err != nil {
		t.Fatal(err)
	}
	pairsAt := len(good) - len(parsed.pairs)
	tableAt := pairsAt - 4 - len(parsed.terms) - 4 - len(parsed.table)
	everyEntry := func(field, stride int, v uint32, from, to int) []byte { // set one u32 of every record
		b := bytes.Clone(good)
		for i := from + field; i < to; i += stride {
			binary.LittleEndian.PutUint32(b[i:], v)
		}
		return reseal(b)
	}
	var oldGob bytes.Buffer
	if err := gob.NewEncoder(&oldGob).Encode(struct{ Schema string }{"go-1"}); err != nil {
		t.Fatal(err)
	}
	flipped := bytes.Clone(good)
	flipped[len(good)/2] ^= 0x40
	for _, tc := range []struct {
		name  string
		data  []byte
		state string
		lazy  bool // parses; the first query that reads the bad part fails
	}{
		{"old gob cache", oldGob.Bytes(), "corrupt", false},
		{"empty", nil, "corrupt", false},
		{"wrong magic", append([]byte("cnavfinX"), good[len(cacheMagic):]...), "corrupt", false},
		{"flipped byte", flipped, "corrupt", false},
		{"truncated", good[:len(good)-3], "corrupt", false},
		{"trailing bytes", reseal(append(bytes.Clone(good), 0)), "corrupt", false},
		{"term offsets out of range", everyEntry(0, termEntrySize, 1<<30, tableAt, tableAt+len(parsed.table)), "corrupt", true},
		{"postings out of range", everyEntry(8, termEntrySize, 1<<30, tableAt, tableAt+len(parsed.table)), "corrupt", true},
		{"posting outside the documents", everyEntry(0, 8, uint32(len(mem.docs)), pairsAt, len(good)), "corrupt", true},
		{"other schema", reseal(bytes.Replace(bytes.Clone(good), []byte(indexSchema), []byte("go-9"), 1)), "stale", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := parseCache(tc.data); tc.lazy != (err == nil) {
				t.Errorf("parseCache: %v", err)
			}
			if err := os.WriteFile(c.IndexPath("docs"), tc.data, 0o644); err != nil {
				t.Fatal(err)
			}
			r := c.Ranked("docs", "alarm escalation", false, nil)
			if !strings.HasPrefix(r.Note, "corpus-nav: find index "+tc.state+" (") || len(r.Results) == 0 {
				t.Errorf("note %q, %d results", r.Note, len(r.Results))
			}
			if again := c.Ranked("docs", "alarm escalation", false, nil); again.Note != "" ||
				!reflect.DeepEqual(scores(again.Results), scores(r.Results)) {
				t.Errorf("rebuilt cache: note %q, results %v want %v", again.Note, scores(again.Results), scores(r.Results))
			}
		})
	}
	if err := os.Remove(c.IndexPath("docs")); err != nil {
		t.Fatal(err)
	}
	if r := c.Ranked("docs", "alarm", false, nil); !strings.HasPrefix(r.Note, "corpus-nav: find index missing (") {
		t.Errorf("missing: %q", r.Note)
	}
}

func scores(rs []Scored) []string {
	var out []string
	for _, r := range rs {
		out = append(out, fmt.Sprintf("%s %.12g", r.Doc.Display, r.Score))
	}
	return out
}
