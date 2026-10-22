package blocksearch

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// TestFindUnlistableRootCannotSearch: a root that cannot be listed when Find lists the
// blocks scope (locked after a caller's entry gate) is "cannot search" (exit 2) naming
// the root and the OS reason, never a miss over 0 files.
func TestFindUnlistableRootCannotSearch(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("mode 0311 does not lock a directory listing on Windows")
	}
	t.Setenv("CORPUS_NAV_FIND_INDEX", "off")
	root := t.TempDir()
	if err := os.CopyFS(root, os.DirFS(filepath.Join("..", "testdata", "corpus"))); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(root, 0o311); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(root, 0o755) })
	if _, err := os.ReadDir(root); err == nil {
		t.Skip("mode 0311 does not stop this user listing a directory (root?)")
	}
	c := New(root, t.TempDir())
	want := "corpus-nav: cannot search: cannot list " + c.Root + ": Permission denied\n"
	for _, scope := range []string{"blocks", "all"} {
		r := c.Find(Query{Text: "alarm", Scope: scope, Limit: 50})
		if r.Exit() != 2 || r.Stdout() != "" || r.Stderr() != want {
			t.Errorf("%s: exit %d\nstdout %q\nstderr %q", scope, r.Exit(), r.Stdout(), r.Stderr())
		}
	}
}

// targetsOf is c.Targets(scope) on a root the test can list.
func targetsOf(c *Corpus, scope string) []Target {
	ts, err := c.Targets(scope)
	if err != nil {
		panic(err)
	}
	return ts
}
