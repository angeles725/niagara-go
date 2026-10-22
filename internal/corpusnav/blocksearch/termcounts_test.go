package blocksearch

import (
	"bytes"
	"encoding/binary"
	"hash/crc32"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
)

// fields is the tokenizer of these tests.
func fields(text string) []string { return strings.Fields(strings.ToLower(text)) }

// naiveCounts is what TermDoc must hold: the terms in first-occurrence order, their counts.
func naiveCounts(tokens []string) ([]string, []int32) {
	var order []string
	counts := map[string]int32{}
	for _, t := range tokens {
		if counts[t] == 0 {
			order = append(order, t)
		}
		counts[t]++
	}
	out := make([]int32, len(order))
	for i, t := range order {
		out[i] = counts[t]
	}
	return order, out
}

func checkCounts(t *testing.T, tc *TermCounts, paths []string) {
	t.Helper()
	if !slices.IsSorted(tc.Vocab) || len(slices.Compact(slices.Clone(tc.Vocab))) != len(tc.Vocab) {
		t.Fatalf("vocabulary not sorted and distinct: %q", tc.Vocab)
	}
	if len(tc.Docs) != len(paths) {
		t.Fatalf("%d docs, want %d", len(tc.Docs), len(paths))
	}
	for i, p := range paths {
		text, ok := ReadText(p)
		d := tc.Docs[i]
		if d.Readable != ok {
			t.Errorf("%s: readable %v, want %v", p, d.Readable, ok)
		}
		order, counts := naiveCounts(fields(text))
		var got []string
		for _, id := range d.IDs {
			got = append(got, tc.Vocab[id])
		}
		if !slices.Equal(got, order) || !slices.Equal(d.Counts, counts) {
			t.Errorf("%s: terms %q counts %v, want %q %v", p, got, d.Counts, order, counts)
		}
		for k, id := range d.IDs {
			if tc.ID(tc.Vocab[id]) != int(id) || d.Counts[k] <= 0 {
				t.Errorf("%s: ID(%q) = %d, want %d", p, tc.Vocab[id], tc.ID(tc.Vocab[id]), id)
			}
		}
	}
	for _, term := range []string{"", "zzzzzz-not-a-term", "\xff"} {
		if id := tc.ID(term); id != -1 {
			t.Errorf("ID(%q) = %d, want -1", term, id)
		}
	}
}

func targetPaths(ts []Target) []string {
	out := make([]string, len(ts))
	for i, t := range ts {
		out[i] = t.Path
	}
	return out
}

// copyFixture is the fixture corpus in a temporary root (its files may be changed).
func copyFixture(t *testing.T) *Corpus {
	t.Helper()
	root := t.TempDir()
	if err := os.CopyFS(root, os.DirFS(filepath.Join("..", "testdata", "corpus"))); err != nil {
		t.Fatal(err)
	}
	return New(root, t.TempDir())
}

// TestCountTerms: every file's terms in first-occurrence order with their counts, ids
// into one sorted vocabulary; a file that cannot be read is an unreadable empty document.
func TestCountTerms(t *testing.T) {
	c := copyFixture(t)
	paths := append(targetPaths(c.Targets("all")), filepath.Join(c.Root, "no-such-file.md"))
	tc := CountTerms(paths, fields)
	checkCounts(t, tc, paths)
	if len(tc.Vocab) < 50 || tc.Docs[len(paths)-1].Readable || len(tc.Docs[len(paths)-1].IDs) != 0 {
		t.Errorf("vocabulary %d terms, last doc %+v", len(tc.Vocab), tc.Docs[len(paths)-1])
	}
}

// TestTermCountsCache walks the cache states: missing (counted and written), hit (the
// same counts), stale after a file changes or another file list, corrupt, off.
func TestTermCountsCache(t *testing.T) {
	t.Setenv("CORPUS_NAV_FIND_INDEX", "")
	c := copyFixture(t)
	targets := c.Targets("blocks")
	paths := targetPaths(targets)
	want := CountTerms(paths, fields)
	get := func(state string, ts []Target, want *TermCounts) {
		t.Helper()
		tc, got := c.CachedTermCounts("tfidf-test", ts, fields)
		if got != state {
			t.Errorf("state %q, want %q", got, state)
		}
		if !reflect.DeepEqual(tc, want) {
			t.Errorf("%s: counts differ from CountTerms", state)
		}
	}
	get("missing", targets, want)
	path := c.TermCountsPath("tfidf-test")
	if st, err := os.Stat(path); err != nil || st.Mode().Perm() != 0o600 && os.PathSeparator == '/' {
		t.Fatalf("cache file: %v %v", st, err)
	}
	get("hit", targets, want)

	if err := os.WriteFile(paths[0], []byte("# Changed\nbrand new words words\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	changed := CountTerms(paths, fields)
	checkCounts(t, changed, paths)
	get("stale", targets, changed)
	get("hit", targets, changed)

	fewer := targets[1:]
	get("stale", fewer, CountTerms(paths[1:], fields))
	get("stale", targets, changed)

	if err := os.WriteFile(path, []byte("not a cache"), 0o644); err != nil {
		t.Fatal(err)
	}
	get("corrupt", targets, changed)
	get("hit", targets, changed)

	t.Setenv("CORPUS_NAV_FIND_INDEX", "off")
	off := New(c.Root, t.TempDir())
	if tc, state := off.CachedTermCounts("tfidf-test", targets, fields); state != "off" || !reflect.DeepEqual(tc, changed) {
		t.Errorf("off: state %q", state)
	}
	if entries, _ := os.ReadDir(off.IndexDir); len(entries) != 0 {
		t.Errorf("off wrote %d file(s)", len(entries))
	}
}

func resealTerms(b []byte) []byte {
	binary.LittleEndian.PutUint32(b[len(termMagic):], crc32.Checksum(b[len(termMagic)+4:], castagnoli))
	return b
}

// TestTermCountsCorrupt: bytes that are not exactly a term-count file of this layout read
// as corrupt, never as other counts.
func TestTermCountsCorrupt(t *testing.T) {
	c := copyFixture(t)
	paths := targetPaths(c.Targets("blocks"))
	tc := CountTerms(paths, fields)
	good := encodeTermCounts(tc, "name", "fp")
	if got, err := parseTermCounts(good, "name", "fp"); err != nil || !reflect.DeepEqual(got, tc) {
		t.Fatalf("round trip: %v", err)
	}
	flipped := bytes.Clone(good)
	flipped[len(good)/2] ^= 0x40
	tampered := func(edit func(*TermCounts)) []byte { // a well-sealed file of counts that cannot be
		bad := &TermCounts{Vocab: slices.Clone(tc.Vocab)}
		for _, d := range tc.Docs {
			bad.Docs = append(bad.Docs, TermDoc{d.Readable, slices.Clone(d.IDs), slices.Clone(d.Counts)})
		}
		edit(bad)
		return encodeTermCounts(bad, "name", "fp")
	}
	last := func(bad *TermCounts) *TermDoc { // the last document with terms
		for i := len(bad.Docs) - 1; ; i-- {
			if len(bad.Docs[i].IDs) > 0 {
				return &bad.Docs[i]
			}
		}
	}
	for name, data := range map[string][]byte{
		"empty": nil, "wrong magic": append([]byte("cnavtermX"), good[len(termMagic):]...), "flipped byte": flipped,
		"truncated": good[:len(good)-3], "trailing bytes": resealTerms(append(bytes.Clone(good), 0)),
		"id out of range":     tampered(func(b *TermCounts) { last(b).IDs[0] = int32(len(b.Vocab)) }),
		"negative id":         tampered(func(b *TermCounts) { last(b).IDs[0] = -1 }),
		"zero count":          tampered(func(b *TermCounts) { last(b).Counts[0] = 0 }),
		"unsorted vocabulary": tampered(func(b *TermCounts) { slices.Reverse(b.Vocab) }),
		"repeated term":       tampered(func(b *TermCounts) { b.Vocab[1] = b.Vocab[0] }),
	} {
		if _, err := parseTermCounts(data, "name", "fp"); err == nil {
			t.Errorf("%s: parsed", name)
		}
	}
	if _, err := parseTermCounts(good, "other", "fp"); err != errTermsStale {
		t.Errorf("other name: %v", err)
	}
	if _, err := parseTermCounts(good, "name", "other"); err != errTermsStale {
		t.Errorf("other fingerprint: %v", err)
	}
}
