package fts

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// fpTree is a data root with two text sources; every folder and file is
// dated an hour back, so no folder is too recent to be trusted (fpRecent).
func fpTree(t *testing.T) string {
	t.Helper()
	base := t.TempDir()
	for rel, text := range map[string]string{
		"guides-clean/a/one.txt":     "one",
		"guides-clean/a/b/two.txt":   "two",
		"guides-clean/skip.bin":      "x",
		"source/m-rt/javax/BX.java":  "class BX {}",
		"source/m-rt/javax/BY.java":  "class BY {}",
		"bajadoc-clean/javax/BZ.txt": "BZ",
	} {
		p := filepath.Join(base, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o777); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(text), 0o666); err != nil {
			t.Fatal(err)
		}
	}
	ageTree(t, base, fpOld)
	return base
}

// fpOld is an hour back: a folder dated then is trusted.
var fpOld = time.Now().Add(-time.Hour).Truncate(time.Second)

// ageTree dates every folder and file under base at old.
func ageTree(t *testing.T, base string, old time.Time) {
	t.Helper()
	var paths []string
	if err := filepath.Walk(base, func(p string, _ os.FileInfo, err error) error {
		paths = append(paths, p)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	for i := len(paths) - 1; i >= 0; i-- { // children before their folder
		if err := os.Chtimes(paths[i], old, old); err != nil {
			t.Fatal(err)
		}
	}
}

// checkCached compares CachedFingerprint with the walking Fingerprint and
// the number of sources it walked again.
func checkCached(t *testing.T, base, step string, wantWalks int) {
	t.Helper()
	before := fpWalks.Load()
	got := CachedFingerprint(base, SourceDirs(base))
	if want := Fingerprint(SourceDirs(base)); got != want {
		t.Errorf("%s: cached fingerprint\n%s\nwant\n%s", step, got, want)
	}
	if n := fpWalks.Load() - before; n != int64(wantWalks) {
		t.Errorf("%s: %d sources walked, want %d", step, n, wantWalks)
	}
}

// The memo gives Fingerprint's value; a source is walked again only when one
// of its folders changed (a file added, removed or renamed), and a file
// changed in place (no folder changes) is still seen through its own stat.
func TestCachedFingerprintFollowsChanges(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	base := fpTree(t)
	guides := filepath.Join(base, "guides-clean")
	checkCached(t, base, "first call", 3)
	checkCached(t, base, "unchanged", 0)

	// In place: same name, same folder mtime, new content and mtime.
	f := filepath.Join(guides, "a", "one.txt")
	dir, _ := os.Stat(filepath.Dir(f))
	if err := os.WriteFile(f, []byte("one, edited in place"), 0o666); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(filepath.Dir(f), dir.ModTime(), dir.ModTime()); err != nil {
		t.Fatal(err)
	}
	checkCached(t, base, "edited in place", 0)
	if err := os.Chtimes(f, fpOld, fpOld); err != nil {
		t.Fatal(err)
	}
	checkCached(t, base, "touched back", 0)

	// A changed listing moves its folder's mtime (here to another old
	// second, as the kernel moves it to now).
	bump := func(dir string, n int) {
		t.Helper()
		at := fpOld.Add(time.Duration(n) * time.Second)
		if err := os.Chtimes(dir, at, at); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(guides, "a", "b", "three.txt"), []byte("3"), 0o666); err != nil {
		t.Fatal(err)
	}
	bump(filepath.Join(guides, "a", "b"), 1)
	checkCached(t, base, "file added", 1)
	javax := filepath.Join(base, "source", "m-rt", "javax")
	if err := os.Remove(filepath.Join(javax, "BY.java")); err != nil {
		t.Fatal(err)
	}
	bump(javax, 2)
	checkCached(t, base, "file removed", 1)
	if err := os.Rename(filepath.Join(guides, "a", "b"), filepath.Join(guides, "a", "c")); err != nil {
		t.Fatal(err)
	}
	bump(filepath.Join(guides, "a"), 3)
	checkCached(t, base, "folder renamed", 1)
	checkCached(t, base, "unchanged again", 0)
}

// A folder modified less than fpRecent ago may change again within the same
// timestamp tick, so its source is walked again until it is older.
func TestCachedFingerprintDistrustsRecentFolders(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	base := fpTree(t)
	checkCached(t, base, "first call", 3)
	now := time.Now()
	if err := os.Chtimes(filepath.Join(base, "guides-clean", "a"), now, now); err != nil {
		t.Fatal(err)
	}
	checkCached(t, base, "recent folder", 1)
	checkCached(t, base, "still recent", 1)
	ageTree(t, base, fpOld.Add(-time.Minute))
	checkCached(t, base, "aged", 3)
	checkCached(t, base, "trusted", 0)
}

// A malformed or foreign memo is ignored: the value is Fingerprint's.
func TestCachedFingerprintIgnoresBadMemo(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	base := fpTree(t)
	checkCached(t, base, "first call", 3)
	memo := fpMemoPath(base)
	if err := os.WriteFile(memo, []byte("not a gob"), 0o600); err != nil {
		t.Fatal(err)
	}
	checkCached(t, base, "malformed memo", 3)
	checkCached(t, base, "rewritten memo", 0)
}
