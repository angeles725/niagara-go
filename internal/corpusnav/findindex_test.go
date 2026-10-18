package corpusnav

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestFindCache walks the cache states: missing (rebuilt, one stderr line), warm (silent,
// same output as in memory), stale after a file changes, corrupt.
func TestFindCache(t *testing.T) {
	t.Setenv("CORPUS_NAV_FIND_INDEX", "")
	c, out, errb := testCorpus(t)
	c.indexDir = t.TempDir()
	want := "docs/guide-alarms.md · Alarm guide\n       1: # Alarm guide\n       3: How alarms are acknowledged.\n\n" +
		"docs/sub/deep.md · Deep doc in a subfolder\n       3: Alarm escalation details.\n" // find alarm --in docs
	run := func(state string) {
		t.Helper()
		out.Reset()
		errb.Reset()
		if code := c.cmdFind("alarm", "docs", 50, false); code != 0 || out.String() != want {
			t.Errorf("%s: exit %d, stdout %q", state, code, out.String())
		}
		note := ""
		if state != "warm" {
			note = "corpus-nav: find index " + state + " (" + c.search().IndexPath("docs") + "); ranked in memory, index rebuilt\n"
		}
		if errb.String() != note {
			t.Errorf("%s: stderr %q, want %q", state, errb.String(), note)
		}
	}
	run("missing")
	run("warm")
	if err := os.WriteFile(c.search().IndexPath("docs"), []byte("not a gob"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("corrupt")
	run("warm")
	docs := t.TempDir()
	if err := os.CopyFS(docs, os.DirFS("testdata/corpus")); err != nil {
		t.Fatal(err)
	}
	c2 := newCorpus(docs, c.indexDir, out, errb)
	c2.out, c2.err = out, errb
	errb.Reset()
	c2.cmdFind("alarm", "docs", 50, false) // another root: different fingerprint
	if !strings.Contains(errb.String(), "find index stale") {
		t.Errorf("another root: %q", errb.String())
	}
	if err := os.WriteFile(filepath.Join(docs, "docs", "guide-alarms.md"), []byte("# Alarm guide\n\nchanged alarm\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	errb.Reset()
	c2.cmdFind("alarm", "docs", 50, false)
	if !strings.Contains(errb.String(), "find index stale") {
		t.Errorf("changed file: %q", errb.String())
	}
}
