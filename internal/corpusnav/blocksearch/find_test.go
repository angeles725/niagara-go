package blocksearch

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestFindMatchesCorpusNavGoldens renders Find as the library result and compares it
// with the corpus-nav find goldens (the Python tool's bytes), so nsearch reading the
// library sees exactly what corpus-nav find prints.
func TestFindMatchesCorpusNavGoldens(t *testing.T) {
	t.Setenv("CORPUS_NAV_FIND_INDEX", "off")
	for _, tc := range []struct {
		golden string
		q      Query
	}{
		{"find-alarm-histories-and", Query{Text: "alarm histories", Scope: "blocks", Limit: 50, RequireAll: true}},
		{"find-alarm-in-all-limit-4", Query{Text: "alarm", Scope: "all", Limit: 4}},
		{"find-escalacion-de-alarmas-limit-3", Query{Text: "escalacion de alarmas", Scope: "blocks", Limit: 3}},
		{"find-zzqq", Query{Text: "zzqq", Scope: "blocks", Limit: 50}},
	} {
		t.Run(tc.golden, func(t *testing.T) {
			if tc.golden == "find-alarm-in-all-limit-4" && runtime.GOOS == "windows" {
				t.Skip("the golden prints docs/ ids; pathlib prints docs\\ on Windows (requires.goos linux)")
			}
			dir := filepath.Join("..", "testdata", "golden", tc.golden)
			r := New(filepath.Join("..", "testdata", "corpus"), t.TempDir()).Find(tc.q)
			if got, want := r.Stdout(), read(t, dir, "stdout"); got != want {
				t.Errorf("stdout:\n%s\nwant:\n%s", got, want)
			}
			if got, want := r.Exit(), strings.TrimSpace(read(t, dir, "exit")); want != "" && got != atoi(t, want) {
				t.Errorf("exit %d, want %s", got, want)
			}
			if r.Stderr() != "" {
				t.Errorf("stderr %q", r.Stderr())
			}
		})
	}
}

// TestFindUnreadableAndCacheTag: an unreadable target fails the find (exit 2, one
// stderr line), and a CacheTag names its own cache file next to corpus-nav's.
func TestFindUnreadableAndCacheTag(t *testing.T) {
	t.Setenv("CORPUS_NAV_FIND_INDEX", "") // the cache is on, whatever the outer environment says
	root := t.TempDir()
	if err := os.CopyFS(root, os.DirFS(filepath.Join("..", "testdata", "corpus"))); err != nil {
		t.Fatal(err)
	}
	bad := filepath.Join(root, "docs", "bad.md")
	if err := os.WriteFile(bad, []byte("alarm \xff\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	c := New(root, t.TempDir())
	c.CacheTag = ".nsearch"
	if got := filepath.Base(c.IndexPath("docs")); got != "find-docs.nsearch.gob" {
		t.Errorf("IndexPath = %s", got)
	}
	r := c.Find(Query{Text: "alarm", Scope: "docs", Limit: 50})
	if r.Exit() != 2 || !r.Found {
		t.Errorf("exit %d found %v", r.Exit(), r.Found)
	}
	wantErr := "corpus-nav: find index missing (" + c.IndexPath("docs") + "); ranked in memory, index rebuilt\n" +
		UnreadableLine(bad) + "\n"
	if r.Stderr() != wantErr {
		t.Errorf("stderr %q, want %q", r.Stderr(), wantErr)
	}
	if _, err := os.Stat(c.IndexPath("docs")); err != nil {
		t.Errorf("tagged cache not written: %v", err)
	}
}

func read(t *testing.T, dir, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	return string(b)
}

func atoi(t *testing.T, s string) int {
	t.Helper()
	n := 0
	for _, r := range s {
		if r < '0' || r > '9' {
			t.Fatalf("exit %q", s)
		}
		n = n*10 + int(r-'0')
	}
	return n
}
