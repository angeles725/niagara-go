package help

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/angeles725/niagara-go/internal/helpbuild"
)

// TestFreshnessReportsRetiredIndexes: an index file the Go port retired (call-index.json,
// decision I3-5) that is still on disk is named, so it can be deleted by hand; freshness
// never deletes it (it only reads the data root) and the result does not depend on it.
func TestFreshnessReportsRetiredIndexes(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	root := t.TempDir()
	for name, body := range map[string]string{
		".manifest.json":  `{"stamped_utc": "2200-01-01T00:00:00+00:00", "sources": {}}`,
		"call-index.json": "{}",
	} {
		if err := os.MkdirAll(filepath.Join(root, "indexes"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, "indexes", name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	var out bytes.Buffer
	if err := (&Env{BaseDir: root, Out: &out}).freshness(); err != nil {
		t.Fatal(err)
	}
	want := "Retired index files still on disk (nh neither reads nor rebuilds them; delete by hand):\n" +
		"  indexes/call-index.json\n"
	if !strings.Contains(out.String(), want) {
		t.Errorf("freshness:\n%s\nwant:\n%s", out.String(), want)
	}
	if _, err := os.Stat(filepath.Join(root, "indexes", "call-index.json")); err != nil {
		t.Errorf("freshness removed the retired index: %v", err)
	}
	status, err := indexStatus(root)
	if err != nil || !strings.HasSuffix(status, "; retired index left: call-index.json") {
		t.Errorf("--all status %q, %v", status, err)
	}
}

// stampedCorpus is a data root with two derived folders (bajadoc-clean, indexes/) stamped
// by rebuild-all --stamp-only at 2030-01-01; touch then sets every file and folder of the
// derived folders to a later mtime without changing a byte, as a git checkout does.
func stampedCorpus(t *testing.T) (root string, touch func()) {
	t.Helper()
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	root = t.TempDir()
	for rel, body := range map[string]string{
		"bajadoc-clean/baja/a.txt": "alpha", "bajadoc-clean/b.txt": "beta", "indexes/x-index.json": "{}",
	} {
		path := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	stamp := time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)
	if _, err := helpbuild.RebuildAll(root, helpbuild.RebuildOptions{StampOnly: true, Now: func() time.Time { return stamp }}); err != nil {
		t.Fatal(err)
	}
	return root, func() {
		later := stamp.Add(time.Hour)
		for _, d := range []string{"bajadoc-clean", "indexes"} {
			err := filepath.WalkDir(filepath.Join(root, d), func(p string, _ os.DirEntry, err error) error {
				if err != nil || filepath.Base(p) == ".manifest.json" {
					return err
				}
				return os.Chtimes(p, later, later)
			})
			if err != nil {
				t.Fatal(err)
			}
		}
	}
}

// freshnessOf is the freshness report of root and its freshness --all index state.
func freshnessOf(t *testing.T, root string) (report, state string) {
	t.Helper()
	var out bytes.Buffer
	if err := (&Env{BaseDir: root, Out: &out}).freshness(); err != nil {
		t.Fatal(err)
	}
	state, err := indexState(root)
	if err != nil {
		t.Fatal(err)
	}
	return out.String(), state
}

// TestFreshnessTouchedUnchanged (#199): derived files rewritten with the same bytes after
// the stamp (a git switch or fast-forward) are FRESH, named "touched, content unchanged";
// the stamp's per-folder content digest decides, not the mtime alone.
func TestFreshnessTouchedUnchanged(t *testing.T) {
	root, touch := stampedCorpus(t)
	touch()
	report, state := freshnessOf(t, root)
	for _, want := range []string{
		"  OK             bajadoc-clean      now: 2 files, 9 bytes — touched, content unchanged\n",
		"  OK             indexes            now: 1 files, 2 bytes — touched, content unchanged\n",
		"RESULT: FRESH — corpus matches the recorded baseline.\n",
	} {
		if !strings.Contains(report, want) {
			t.Errorf("freshness:\n%s\nwant:\n%s", report, want)
		}
	}
	if state != "FRESH" {
		t.Errorf("--all index state %q, want FRESH", state)
	}
}

// TestFreshnessTouchedChanged: one changed byte (same size) under a touched folder is STALE.
func TestFreshnessTouchedChanged(t *testing.T) {
	root, touch := stampedCorpus(t)
	if err := os.WriteFile(filepath.Join(root, "bajadoc-clean", "b.txt"), []byte("bets"), 0o644); err != nil {
		t.Fatal(err)
	}
	touch()
	report, state := freshnessOf(t, root)
	for _, want := range []string{
		"  CHANGED        bajadoc-clean      now: 2 files, 9 bytes — modified after the baseline was stamped\n",
		"  OK             indexes            now: 1 files, 2 bytes — touched, content unchanged\n",
		"RESULT: STALE — 1 source(s) changed: bajadoc-clean\n",
	} {
		if !strings.Contains(report, want) {
			t.Errorf("freshness:\n%s\nwant:\n%s", report, want)
		}
	}
	if state != "STALE" {
		t.Errorf("--all index state %q, want STALE", state)
	}
}

// TestFreshnessTouchedNoDigest: a baseline stamped before the digests (no "derived" key)
// keeps the mtime rule: a touched folder is CHANGED even when no byte changed.
func TestFreshnessTouchedNoDigest(t *testing.T) {
	root, touch := stampedCorpus(t)
	path := filepath.Join(root, "indexes", ".manifest.json")
	if err := os.WriteFile(path, []byte(`{"stamped_utc": "2030-01-01T00:00:00+00:00", "sources": {}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	touch()
	report, state := freshnessOf(t, root)
	for _, want := range []string{
		"  CHANGED        bajadoc-clean      now: 2 files, 9 bytes — modified after the baseline was stamped\n",
		"RESULT: STALE — 2 source(s) changed: bajadoc-clean, indexes\n",
	} {
		if !strings.Contains(report, want) {
			t.Errorf("freshness:\n%s\nwant:\n%s", report, want)
		}
	}
	if state != "STALE" {
		t.Errorf("--all index state %q, want STALE", state)
	}
}
