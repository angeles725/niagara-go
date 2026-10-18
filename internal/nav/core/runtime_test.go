package core

import (
	"bytes"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func newTestRuntime(t *testing.T) (*Runtime, *bytes.Buffer, *bytes.Buffer, string) {
	t.Helper()
	base := t.TempDir()
	if err := os.MkdirAll(filepath.Join(base, "indexes"), 0o755); err != nil {
		t.Fatal(err)
	}
	var out, errb bytes.Buffer
	return NewAt("", base, &out, &errb), &out, &errb, base
}

func writeIndex(t *testing.T, base, name, body string) {
	t.Helper()
	if err := os.WriteFile(IndexPath(base, name), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestOutcomeHighestStatusWins(t *testing.T) {
	r, out, errb, _ := newTestRuntime(t)
	r.Miss("class 'X'", "3 class entries", "search '*X*'")
	r.Error("class index not found: /p", "Run: ./reindex.sh")
	r.Ambiguous()
	if r.ExitCode() != StatusError {
		t.Errorf("exit %d, want 2", r.ExitCode())
	}
	if out.String() != "  0 matches for class 'X' in 3 class entries.\n  Try: search '*X*'\n" {
		t.Errorf("stdout %q", out.String())
	}
	if errb.String() != "ERROR: class index not found: /p\n  Run: ./reindex.sh\n" {
		t.Errorf("stderr %q", errb.String())
	}
	r2, _, _, _ := newTestRuntime(t)
	r2.Ambiguous()
	if r2.ExitCode() != StatusMiss {
		t.Errorf("ambiguous: exit %d", r2.ExitCode())
	}
}

// TestLoadMessages pins the stderr of indexes.load, captured from Python
// (progress line interrupted by the error, unreadable and missing indexes, a
// NOTE once for an absent optional index).
func TestLoadMessages(t *testing.T) {
	r, _, errb, base := newTestRuntime(t)
	big := `{"_meta": {"timestamp": "T"}, "classes": {` + strings.Repeat(" ", 21*1024*1024) + "\n  \"x\": [1,]}}"
	writeIndex(t, base, "class-index.json", big)
	writeIndex(t, base, "module-inventory.json", `{"modules": {}`)
	for _, name := range []string{"class-index.json", "module-inventory.json", "missing-index.json"} {
		if r.Load(name, LoadOpts{}) != nil {
			t.Errorf("%s loaded", name)
		}
	}
	for range 2 {
		if r.Load("opt-index.json", LoadOpts{Optional: true}) != nil {
			t.Error("optional index loaded")
		}
	}
	idx := filepath.Join(base, "indexes")
	in := func(name string) string { return filepath.Join(idx, name) }
	want := "  Loading class-index.json (21.0 MB)...ERROR: class index is unreadable: " + in("class-index.json") +
		" (Illegal trailing comma before end of array: line 2 column 10 (char 22020148))\n" +
		"  Run: ./reindex.sh\n\n" +
		"ERROR: module inventory is unreadable: " + in("module-inventory.json") + " (Expecting ',' delimiter: line 1 column 15 (char 14))\n" +
		"  Run: ./reindex.sh\n" +
		"ERROR: missing index not found: " + in("missing-index.json") + "\n  Run: ./reindex.sh\n" +
		"NOTE: optional opt index not found (" + in("opt-index.json") + "); the output omits what it adds. Build it with ./reindex.sh\n"
	if errb.String() != want {
		t.Errorf("stderr:\n%s\nwant:\n%s", errb.String(), want)
	}
	if r.ExitCode() != StatusError {
		t.Errorf("exit %d", r.ExitCode())
	}
}

func TestLoadCachesAndPrintsProgress(t *testing.T) {
	r, _, errb, base := newTestRuntime(t)
	t.Setenv("NAV_CORPUS_BASE", filepath.Join(base, "no-corpus"))
	writeIndex(t, base, "class-index.json", `{"_meta": {}, "classes": {}`+strings.Repeat(" ", 20*1024*1024)+"}")
	a := r.Load("class-index.json", LoadOpts{})
	b := r.Load("class-index.json", LoadOpts{})
	if a == nil || a != b {
		t.Fatal("not loaded once and shared")
	}
	if errb.String() != "  Loading class-index.json (20.0 MB)... OK\n" {
		t.Errorf("stderr %q", errb.String())
	}
	r2, _, errb2, _ := newTestRuntime(t)
	r2.BaseDir = base
	if r2.Load("class-index.json", LoadOpts{Quiet: true}) == nil || errb2.Len() != 0 {
		t.Errorf("quiet load printed %q", errb2.String())
	}
}

func TestStalenessWarnings(t *testing.T) {
	root := t.TempDir()
	organized := filepath.Join(root, "organized")
	for _, d := range []string{"alarm/alarm-rt/vineflower", "baja/baja/vineflower"} {
		if err := os.MkdirAll(filepath.Join(organized, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("NAV_CORPUS_BASE", organized)
	idx := filepath.Join(root, "class-index.json")
	if err := os.WriteFile(idx, []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	if w := StalenessAt(OrganizedDir(), nil, idx); len(w) != 0 {
		t.Errorf("fresh index warned: %q", w)
	}
	old := root + "/old-organized"
	meta := NewTestMeta(t, `{"source": `+strconv.Quote(old)+`}`) // a JSON string on Windows too
	past := mustMtime(t, idx) - 100
	setMtime(t, idx, past)
	w := StalenessAt(OrganizedDir(), meta, idx)
	want := []string{
		"index class-index.json was built from " + old + " but the corpus is " + organized,
		"3 corpus modules had directories added, removed or re-created after class-index.json was built (e.g. organized/, alarm, baja)",
		"rebuild with ./reindex.sh",
	}
	if strings.Join(w, "\n") != strings.Join(want, "\n") {
		t.Errorf("warnings:\n%s\nwant:\n%s", strings.Join(w, "\n"), strings.Join(want, "\n"))
	}
}
