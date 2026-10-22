package blocksearch

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/angeles725/niagara-go/internal/pycompat"
)

// Equivalence with the code before internal/statcache (PR #179): the legacy*
// functions are that code verbatim (legacyStatStamp per platform, in the _linux and
// _other files), and every case must decide as it did. The find and term-count caches
// are keyed by fingerprint, which hashes statStamp: an equal fingerprint is an equal
// cache, byte for byte.

func legacyIndexDisabled() bool {
	switch strings.ToLower(pycompat.Strip(os.Getenv("CORPUS_NAV_FIND_INDEX"))) {
	case "off", "0", "no", "false":
		return true
	}
	return false
}

// legacyFingerprint is Corpus.fingerprint (unchanged by PR #179) over legacyStatStamp.
func legacyFingerprint(c *Corpus, targets []Target) string {
	h := sha256.New()
	h.Write([]byte(indexSchema))
	if exe, err := os.Executable(); err == nil {
		fmt.Fprintf(h, "\x00%s", legacyStatStamp(exe))
	}
	for _, t := range append([]Target{{"CATALOG", c.Catalog}}, targets...) {
		fmt.Fprintf(h, "%s\x00%s\x00%s\n", t.Display, t.Path, legacyStatStamp(t.Path))
	}
	return hex.EncodeToString(h.Sum(nil))
}

func TestIndexDisabledMatchesLegacy(t *testing.T) {
	for _, sw := range []string{"", "off", " OFF ", "\tno\n", "False", "0", "1", "on", "offline", "of f", " off"} {
		t.Setenv("CORPUS_NAV_FIND_INDEX", sw)
		if got, want := indexDisabled(), legacyIndexDisabled(); got != want {
			t.Errorf("switch %q: %v, legacy %v", sw, got, want)
		}
	}
}

func TestStatStampMatchesLegacy(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "a.md")
	if err := os.WriteFile(file, []byte("text"), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "link.md")
	if err := os.Symlink(file, link); err != nil {
		t.Fatal(err)
	}
	dangling := filepath.Join(dir, "dangling.md")
	if err := os.Symlink(filepath.Join(dir, "gone"), dangling); err != nil {
		t.Fatal(err)
	}
	exe, _ := os.Executable()
	for _, p := range []string{file, link, dangling, dir, filepath.Join(dir, "missing"), exe} {
		if got, want := statStamp(p), legacyStatStamp(p); got != want {
			t.Errorf("%s: stamp %q, legacy %q", p, got, want)
		}
	}
}

// TestCacheBytesMatchLegacy: the find cache and the term-count cache written through
// the migrated fingerprint are the bytes the legacy fingerprint would have written.
func TestCacheBytesMatchLegacy(t *testing.T) {
	t.Setenv("CORPUS_NAV_FIND_INDEX", "")
	c := copyFixture(t)
	for _, scope := range []string{"blocks", "all"} {
		targets := targetsOf(c, scope)
		fp := legacyFingerprint(c, targets)
		if got := c.fingerprint(targets); got != fp {
			t.Fatalf("%s: fingerprint %s, legacy %s", scope, got, fp)
		}
		if r := c.Ranked(scope, "alarm", false, nil); r.Fallback != "missing" {
			t.Fatalf("%s: find cache %q, want missing (then written)", scope, r.Fallback)
		}
		got, err := os.ReadFile(c.IndexPath(scope))
		if err != nil {
			t.Fatal(err)
		}
		legacy := filepath.Join(t.TempDir(), "find.gob")
		if err := writeCacheFile(c.NewMemIndex(targets), fp, legacy); err != nil {
			t.Fatal(err)
		}
		if want, _ := os.ReadFile(legacy); !bytes.Equal(got, want) {
			t.Errorf("%s: find cache bytes differ from the legacy fingerprint's", scope)
		}
		if r := c.Ranked(scope, "alarm", false, nil); r.Fallback != "" {
			t.Errorf("%s: warm find %q, want a cache hit", scope, r.Fallback)
		}
		tc, state := c.CachedTermCounts("tfidf-"+scope, targets, fields)
		if state != "missing" {
			t.Fatalf("%s: term counts %q, want missing (then written)", scope, state)
		}
		data, err := os.ReadFile(c.TermCountsPath("tfidf-" + scope))
		if err != nil {
			t.Fatal(err)
		}
		if want := encodeTermCounts(tc, "tfidf-"+scope, fp); !bytes.Equal(data, want) {
			t.Errorf("%s: term-count cache bytes differ from the legacy fingerprint's", scope)
		}
		if _, state := c.CachedTermCounts("tfidf-"+scope, targets, fields); state != "hit" {
			t.Errorf("%s: warm term counts %q, want hit", scope, state)
		}
	}
}
