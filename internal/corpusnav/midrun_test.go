package corpusnav

import (
	"os"
	"runtime"
	"strings"
	"testing"
)

// lockListing makes root unlistable while its files stay readable by path (mode 0311:
// search, no read), the state of a root locked after Main's entry gate listed it.
func lockListing(t *testing.T, root string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("mode 0311 does not lock a directory listing on Windows")
	}
	if err := os.Chmod(root, 0o311); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(root, 0o755) })
	if _, err := os.ReadDir(root); err == nil {
		t.Skip("mode 0311 does not stop this user listing a directory (root?)")
	}
}

// TestRootLockedMidRunKeepsTheGateListing: the main blocks are listed once, by the entry
// gate that checks the error; a root locked after it does not turn into a corpus of zero
// blocks for the commands that run past the gate.
func TestRootLockedMidRunKeepsTheGateListing(t *testing.T) {
	c, out, errb, root := copyCorpus(t)
	if len(c.blocks()) == 0 { // the entry gate's listing
		t.Fatal("the fixture corpus has no main block")
	}
	lockListing(t, root)
	if code := c.cmdShow(4); code != 0 || !strings.HasPrefix(out.String(), "B4 · ") || errb.String() != "" {
		t.Errorf("show 4: exit %d\nstdout %q\nstderr %q", code, out.String(), errb.String())
	}
}

// TestGrepRootLockedMidRunCannotSearch: a search scope listed after the gate, on a root
// that can no longer be listed, is "cannot search" (exit 2), not "no matches" over 0 files.
func TestGrepRootLockedMidRunCannotSearch(t *testing.T) {
	c, out, errb, root := copyCorpus(t)
	lockListing(t, root)
	want := "corpus-nav: cannot search: cannot list " + pyPath(root) + ": Permission denied\n"
	for _, scope := range []string{"blocks", "all"} {
		out.Reset()
		errb.Reset()
		if code := c.cmdGrep("alarm", scope, 50); code != 2 || out.String() != "" || errb.String() != want {
			t.Errorf("grep --in %s: exit %d\nstdout %q\nstderr %q", scope, code, out.String(), errb.String())
		}
	}
}

// TestFocusesRootLockedMidRunCannotSearch: the focuses are the root's RESEARCH-STATE
// files and the focus blocks include the sub-corpus walked from the root; a root that can
// no longer be listed has unknown focuses (exit 2 naming the root and the OS reason),
// never "No blocks found", "Known focuses: " or "focuses: 0".
func TestFocusesRootLockedMidRunCannotSearch(t *testing.T) {
	c, out, errb, root := copyCorpus(t)
	c.blocks() // the entry gate's listing
	lockListing(t, root)
	cannot := "corpus-nav: cannot search: cannot list " + pyPath(root) + ": Permission denied\n"
	if code := c.cmdByFocus("nope"); code != 2 || out.String() != "" || errb.String() != cannot {
		t.Errorf("by-focus nope: exit %d\nstdout %q\nstderr %q", code, out.String(), errb.String())
	}
	out.Reset()
	errb.Reset()
	if code := c.cmdStats(); code != 2 || out.String() != "" || errb.String() != cannot {
		t.Errorf("stats: exit %d\nstdout %q\nstderr %q", code, out.String(), errb.String())
	}
}

// TestSubCorpusRootLockedMidRunCannotSearch: the sub-corpus is walked from the root after
// the gate; a root that can no longer be listed is "cannot search" (exit 2 naming the root
// and the OS reason) with nothing on stdout, never "+ 0 sub-corpus block(s)".
func TestSubCorpusRootLockedMidRunCannotSearch(t *testing.T) {
	c, out, errb, root := copyCorpus(t)
	c.blocks() // the entry gate's listing
	lockListing(t, root)
	want := "corpus-nav: cannot search: cannot list " + pyPath(root) + ": Permission denied\n"
	for _, focus := range []string{"", "alarms"} {
		out.Reset()
		errb.Reset()
		if code := c.cmdList(focus); code != 2 || out.String() != "" || errb.String() != want {
			t.Errorf("list %q: exit %d\nstdout %q\nstderr %q", focus, code, out.String(), errb.String())
		}
	}
}
