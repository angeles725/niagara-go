package fts

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func writeTree(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for rel, body := range files {
		path := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func hexSum(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

// TestDerivedNames: the full-text sources but the raw ones, then indexes/.
func TestDerivedNames(t *testing.T) {
	got := DerivedNames([]string{"bajadoc", "source", "guides", "devguide", "docs"})
	want := []string{"bajadoc-clean", "devguide-clean", "guides-clean", "docs-text", "course-text",
		"devguide-rc", "devguide-examples", "package-text", "lexicon-text", "module-examples", "indexes"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("DerivedNames = %q, want %q", got, want)
	}
}

// TestDerivedDigest pins the digest formula (one "<rel>\x00<sha256>\n" line per file in
// walk order), the indexes/ selection (top-level JSON but the manifest and the retired
// indexes) and that a renamed or changed file changes it; an absent folder has none.
func TestDerivedDigest(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, map[string]string{
		"bajadoc-clean/b.txt": "beta", "bajadoc-clean/baja/a.txt": "alpha",
		"indexes/x-index.json": "{}", "indexes/.manifest.json": "{}", "indexes/call-index.json": "{}",
		"indexes/fulltext.sqlite": "db", "indexes/sub/y.json": "{}",
	})
	want := hexSum("b.txt\x00" + hexSum("beta") + "\n" + "baja/a.txt\x00" + hexSum("alpha") + "\n")
	if got, ok := DerivedDigest(root, "bajadoc-clean"); !ok || got != want {
		t.Errorf("bajadoc-clean digest %q, %v; want %q", got, ok, want)
	}
	if got, ok := DerivedDigest(root, "indexes"); !ok || got != hexSum("x-index.json\x00"+hexSum("{}")+"\n") {
		t.Errorf("indexes digest %q, %v: only x-index.json counts", got, ok)
	}
	if _, ok := DerivedDigest(root, "guides-clean"); ok {
		t.Error("an absent folder has a digest")
	}
	before, _ := DerivedDigest(root, "bajadoc-clean")
	if err := os.Rename(filepath.Join(root, "bajadoc-clean", "b.txt"), filepath.Join(root, "bajadoc-clean", "c.txt")); err != nil {
		t.Fatal(err)
	}
	if after, _ := DerivedDigest(root, "bajadoc-clean"); after == before {
		t.Error("a renamed file kept the digest")
	}
	writeTree(t, root, map[string]string{"bajadoc-clean/c.txt": "bets"})
	if after, _ := DerivedDigest(root, "bajadoc-clean"); after == before {
		t.Error("a changed byte kept the digest")
	}
}
