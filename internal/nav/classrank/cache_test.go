package classrank

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// cachedBase is a base directory whose ranker is cached (no size floor), with
// a _meta naming another corpus so that the staleness warning is part of stderr.
func cachedBase(t *testing.T, withXref bool) string {
	t.Helper()
	old := cacheMinBytes
	cacheMinBytes = 0
	t.Cleanup(func() { cacheMinBytes = old })
	ci := strings.Replace(string(skipsIndex(t)), `"_meta": {`,
		`"_meta": {"source": "/elsewhere/organized", "n": 1.50, `, 1)
	files := map[string][]byte{classIndex: []byte(ci), "class-rank.cache": []byte("python's")}
	if withXref {
		files[xrefIndex] = []byte(`{"class_imported_by": {"BServiceAlarm": ["A", "B", "A"]}}`)
	}
	return newBase(t, files)
}

// TestCacheHitPrintsWhatABuildPrints runs rank twice (build, then hit) with and
// without an xref index and compares both streams; Python's cache is left alone.
func TestCacheHitPrintsWhatABuildPrints(t *testing.T) {
	for _, withXref := range []bool{true, false} {
		base := cachedBase(t, withXref)
		out1, err1, code1 := run(t, base, "alarm service", 2)
		st, err := os.Stat(SourceOf(base).Cache)
		if err != nil {
			t.Fatalf("xref=%v: no cache written: %v", withXref, err)
		}
		if st.Mode().Perm() != 0o600 {
			t.Errorf("cache mode %v, want 0600", st.Mode().Perm())
		}
		out2, err2, code2 := run(t, base, "alarm service", 2)
		if out1 != out2 || err1 != err2 || code1 != code2 {
			t.Errorf("xref=%v: hit differs from build:\n%q %q\n%q %q", withXref, out1, err1, out2, err2)
		}
		if !strings.Contains(err1, "WARNING: index class-index.json was built from /elsewhere/organized") {
			t.Errorf("xref=%v: staleness warning missing: %q", withXref, err1)
		}
		if withXref == strings.Contains(err1, "NOTE: optional xref index") {
			t.Errorf("xref=%v: NOTE mismatch: %q", withXref, err1)
		}
		if b, _ := os.ReadFile(filepath.Join(base, "indexes", "class-rank.cache")); string(b) != "python's" {
			t.Errorf("Python's cache was touched: %q", b)
		}
	}
}

// TestCacheRebuildsWhenStaleOrCorrupt: a corrupt cache and a changed index are
// both ignored, the ranker rebuilt and the cache rewritten.
func TestCacheRebuildsWhenStaleOrCorrupt(t *testing.T) {
	base := cachedBase(t, true)
	want, _, _ := run(t, base, "alarm", 20)
	path := SourceOf(base).Cache
	if err := os.WriteFile(path, []byte("garbage"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got, _, _ := run(t, base, "alarm", 20); got != want {
		t.Errorf("after corruption: %q, want %q", got, want)
	}
	if b, _ := os.ReadFile(path); string(b) == "garbage" {
		t.Error("corrupt cache not rewritten")
	}
	xref := filepath.Join(base, "indexes", xrefIndex)
	if err := os.WriteFile(xref, []byte(`{"class_imported_by": {"ServiceAlarm": ["A"]}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	future := time.Now().Add(time.Hour)
	if err := os.Chtimes(xref, future, future); err != nil {
		t.Fatal(err)
	}
	got, _, _ := run(t, base, "alarm", 20)
	if got == want || !strings.Contains(got, "1\t0.273\tServiceAlarm") {
		t.Errorf("changed xref index not picked up:\n%s", got)
	}
}

// TestCacheUnwritableIsSkipped: a read-only indexes/ never fails rank.
func TestCacheUnwritableIsSkipped(t *testing.T) {
	base := cachedBase(t, true)
	dir := filepath.Join(base, "indexes")
	if err := os.Chmod(dir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(dir, 0o755) })
	if out, _, code := run(t, base, "alarm", 20); code != 0 || !strings.Contains(out, "Rank 'alarm'") {
		t.Errorf("exit %d, stdout %q", code, out)
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 3 {
		t.Errorf("indexes/ holds %d entries, want the 3 written by the test", len(entries))
	}
}
