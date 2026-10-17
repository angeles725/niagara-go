package corpusnav

import (
	"bytes"
	"os"
	"testing"
)

// copyCorpus copies the fixture corpus to a temporary root the test may alter.
func copyCorpus(t *testing.T) (*corpus, *bytes.Buffer, *bytes.Buffer, string) {
	t.Helper()
	root := t.TempDir()
	if err := os.CopyFS(root, os.DirFS("testdata/corpus")); err != nil {
		t.Fatal(err)
	}
	var out, errb bytes.Buffer
	return newCorpus(root, "", &out, &errb), &out, &errb, root
}

// TestByFocusNamesUnreadable: Python's blocks_for_focus skips a block it cannot read
// (and a RESEARCH-STATE file it cannot read raises a traceback). Intended difference:
// each one is named on stderr and the run exits 2, found or not, as by-marker does.
func TestByFocusNamesUnreadable(t *testing.T) {
	c, out, errb, root := copyCorpus(t)
	if err := os.Mkdir(root+"/niagara-mental-model-bloque9.md", 0o755); err != nil {
		t.Fatal(err)
	}
	want := "B4 · Enrutamiento de alarmas\nB5 · Histories and trend logs\nB6 · Security incident review\n" +
		"R2 · Block R2 — Reflow licensing\n\nTotal: 4 block(s) in focus 'alpha'.\n"
	warn := "corpus-nav: cannot read B9 (missing, unreadable or not UTF-8)\n"
	if code := c.cmdByFocus("alpha"); code != 2 || out.String() != want || errb.String() != warn {
		t.Errorf("alpha: exit %d\nstdout %q\nstderr %q", code, out.String(), errb.String())
	}

	c, out, errb, root = copyCorpus(t)
	state := root + "/RESEARCH-STATE-gamma.md"
	if err := os.WriteFile(state, []byte("cites [B4] \xff\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	warn = "corpus-nav: cannot read " + state + " (missing, unreadable or not UTF-8)\n" +
		"No blocks found for focus 'gamma'.\nKnown focuses: alpha, gamma\n"
	if code := c.cmdByFocus("gamma"); code != 2 || out.String() != "" || errb.String() != warn {
		t.Errorf("gamma: exit %d\nstdout %q\nstderr %q", code, out.String(), errb.String())
	}
}

// TestListFocusNamesUnreadable: list --focus shares blocks_for_focus, so it reports an
// unreadable block and exits 2 too.
func TestListFocusNamesUnreadable(t *testing.T) {
	c, out, errb, root := copyCorpus(t)
	if err := os.Mkdir(root+"/niagara-mental-model-bloque9.md", 0o755); err != nil {
		t.Fatal(err)
	}
	if code := c.cmdList("alpha"); code != 2 || errb.String() != "corpus-nav: cannot read B9 (missing, unreadable or not UTF-8)\n" ||
		!bytes.HasSuffix(out.Bytes(), []byte("\nTotal: 3 block(s) + 1 sub-corpus block(s) in focus 'alpha'\n")) {
		t.Errorf("list --focus alpha: exit %d\nstdout %q\nstderr %q", code, out.String(), errb.String())
	}
}
