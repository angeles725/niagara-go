package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/angeles725/niagara-go/internal/cli"
	"github.com/angeles725/niagara-go/internal/help"
	"github.com/angeles725/niagara-go/internal/helpbuild"
)

// freshnessRoot is a data root with a raw source, two derived text sources and an
// index, stamped by rebuild-all --stamp-only (helpbuild writes the baseline, help
// reads it: this test is where the two meet).
func freshnessRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	for rel, body := range map[string]string{
		"source/a/B.java": "class B {}", "lexicon-text/kit/a.txt": "lexicon", "package-text/p.txt": "package",
		"indexes/class-index.json": "{}",
	} {
		write(t, filepath.Join(root, filepath.FromSlash(rel)), body, time.Unix(1_700_000_000, 0))
	}
	if _, err := helpbuild.RebuildAll(root, helpbuild.RebuildOptions{StampOnly: true}); err != nil {
		t.Fatal(err)
	}
	return root
}

func write(t *testing.T, path, body string, mtime time.Time) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, mtime, mtime); err != nil {
		t.Fatal(err)
	}
}

func freshness(t *testing.T, root string) string {
	t.Helper()
	t.Setenv(help.EnvDir, root)
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	var out, errb bytes.Buffer
	if code := help.Main([]string{"freshness"}, cli.Stdio{In: strings.NewReader(""), Out: &out, Err: &errb}); code != 0 {
		t.Fatalf("freshness exit %d: %s%s", code, errb.String(), out.String())
	}
	return out.String()
}

// TestFreshnessSeesDerivedChanges: a derived text source or index written after the
// baseline was stamped is not FRESH: freshness checks every text source of the full-text
// index and the indexes against the stamp, not only the five raw folders.
func TestFreshnessSeesDerivedChanges(t *testing.T) {
	for _, c := range []struct{ name, rel, stale string }{
		{"lexicon text rewritten", "lexicon-text/kit/a.txt", "lexicon-text"},
		{"package text added", "package-text/q.txt", "package-text"},
		{"index rewritten", "indexes/class-index.json", "indexes"},
	} {
		t.Run(c.name, func(t *testing.T) {
			root := freshnessRoot(t)
			if got := freshness(t, root); !strings.Contains(got, "RESULT: FRESH") {
				t.Fatalf("just stamped:\n%s", got)
			}
			path := filepath.Join(root, filepath.FromSlash(c.rel))
			body := "LEXICON"
			if c.stale == "indexes" {
				body = "[]"
			}
			write(t, path, body, time.Now().Add(time.Minute)) // after the stamp
			got := freshness(t, root)
			if !strings.Contains(got, "RESULT: STALE") || !strings.Contains(got, c.stale) {
				t.Errorf("after %s:\n%s", c.name, got)
			}
		})
	}
}
