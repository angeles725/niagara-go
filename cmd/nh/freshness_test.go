package main

import (
	"bytes"
	"os"
	"path/filepath"
	"regexp"
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

// freshness runs `nh freshness` on root: its stdout, stderr and exit status.
func freshness(t *testing.T, root string) (stdout, stderr string, code int) {
	t.Helper()
	t.Setenv(help.EnvDir, root)
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	var out, errb bytes.Buffer
	code = help.Main([]string{"freshness"}, cli.Stdio{In: strings.NewReader(""), Out: &out, Err: &errb})
	return out.String(), errb.String(), code
}

// TestFreshnessExitStatus: a script can gate on the exit status alone: FRESH is 0,
// STALE 1 (the corpus was checked and is out of date, as a miss), and a corpus that
// could not be checked 2: UNKNOWN (no readable stamp time, no baseline) on stdout, an
// unreadable manifest as "ERROR: ..." on stderr.
func TestFreshnessExitStatus(t *testing.T) {
	root := freshnessRoot(t)
	if out, errs, code := freshness(t, root); code != cli.ExitFound || !strings.Contains(out, "RESULT: FRESH") {
		t.Errorf("fresh: exit %d, want 0:\n%s%s", code, out, errs)
	}
	manifest := filepath.Join(root, "indexes", ".manifest.json")
	stamped, err := os.ReadFile(manifest)
	if err != nil {
		t.Fatal(err)
	}
	unstamped := regexp.MustCompile(`"stamped_utc": *"[^"]*"`).ReplaceAll(stamped, []byte(`"stamped_utc": "?"`))
	if err := os.WriteFile(manifest, unstamped, 0o644); err != nil || bytes.Equal(unstamped, stamped) {
		t.Fatalf("manifest without a readable stamp time: %v\n%s", err, stamped)
	}
	if out, errs, code := freshness(t, root); code != cli.ExitCannotSearch || !strings.Contains(out, "RESULT: UNKNOWN") {
		t.Errorf("no readable stamp time: exit %d, want 2:\n%s%s", code, out, errs)
	}
	if err := os.WriteFile(manifest, stamped, 0o644); err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(root, "package-text", "q.txt"), "new", time.Now().Add(time.Minute))
	if out, errs, code := freshness(t, root); code != cli.ExitNoMatch || !strings.Contains(out, "RESULT: STALE") {
		t.Errorf("stale: exit %d, want 1:\n%s%s", code, out, errs)
	}
	if err := os.WriteFile(manifest, []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if out, errs, code := freshness(t, root); code != cli.ExitCannotSearch || !strings.HasPrefix(errs, "ERROR: ") {
		t.Errorf("unreadable manifest: exit %d, want 2 and ERROR:\n%s%s", code, out, errs)
	}
	if err := os.Remove(manifest); err != nil {
		t.Fatal(err)
	}
	if out, errs, code := freshness(t, root); code != cli.ExitCannotSearch || !strings.Contains(out, "UNKNOWN — no baseline manifest recorded.") {
		t.Errorf("no baseline: exit %d, want 2:\n%s%s", code, out, errs)
	}
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
			if got, _, code := freshness(t, root); code != 0 || !strings.Contains(got, "RESULT: FRESH") {
				t.Fatalf("just stamped:\n%s", got)
			}
			path := filepath.Join(root, filepath.FromSlash(c.rel))
			body := "LEXICON"
			if c.stale == "indexes" {
				body = "[]"
			}
			write(t, path, body, time.Now().Add(time.Minute)) // after the stamp
			got, _, code := freshness(t, root)
			if code != 1 || !strings.Contains(got, "RESULT: STALE") || !strings.Contains(got, c.stale) {
				t.Errorf("after %s: exit %d\n%s", c.name, code, got)
			}
		})
	}
}
