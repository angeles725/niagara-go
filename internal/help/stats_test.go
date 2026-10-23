package help

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// statsRoot is a data root outside any registry holding the nine indexes stats reads, each
// an empty JSON object, but the one named skip.
func statsRoot(t *testing.T, skip string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "indexes"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, n := range statsIndexNames {
		if n != skip {
			if err := os.WriteFile(filepath.Join(dir, "indexes", n+".json"), []byte("{}"), 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}
	return dir
}

// TestStatsUnregisteredRoot: a data root no registry names prints "unregistered" as its
// version in the text report, and every count of an empty corpus is zero.
func TestStatsUnregisteredRoot(t *testing.T) {
	dir := statsRoot(t, "")
	var out bytes.Buffer
	if err := (&Env{BaseDir: dir, Out: &out}).Stats(false); err != nil {
		t.Fatal(err)
	}
	want := `Niagara Help Navigator - Corpus Statistics
=======================================================
Niagara version: unregistered  (<DIR>)

INDEXES (9):

  class-index.json            0 classes (0 packages)
  devguide-toc.json           0 guides
  guides-index.json           0 folders
  inheritance.json            0 entries
  pdf-index.json              0 PDFs
  method-index.json           0 names (0 entries)
  source-index.json           0 classes (0 lines)
  xref-index.json             0 classes (0 edges)
  slots-index.json            0 classes (0 props, 0 actions, 0 topics)

TEXT SOURCES (0.0 MB total):

  bajadoc-clean/         0 files  (0.0 MB)
  devguide-clean/        0 files  (0.0 MB)
  guides-clean/          0 files  (0.0 MB)
  docs-text/             0 files  (0.0 MB)
  source/                0 files  (0.0 MB)

RAW HTML:

  bajadoc/               0 HTML
  devguide/              0 HTML
  guides/                0 HTML

INDEXES: 9
CLI: 19 commands
`
	if got := strings.ReplaceAll(out.String(), dir, "<DIR>"); got != want {
		t.Errorf("stats:\n%s\nwant:\n%s", got, want)
	}
}

// TestStatsMissingIndex: an index stats needs that is not built is a *MissingIndexError
// naming it, and nothing is printed.
func TestStatsMissingIndex(t *testing.T) {
	for _, skip := range []string{"class-index", "slots-index"} {
		dir := statsRoot(t, skip)
		var out bytes.Buffer
		err := (&Env{BaseDir: dir, Out: &out}).Stats(false)
		var miss *MissingIndexError
		if !errors.As(err, &miss) || miss.Name != skip+".json" || out.Len() != 0 {
			t.Errorf("missing %s: err %v, stdout %q; want a MissingIndexError naming it and no output", skip, err, out.String())
		}
	}
}

// TestStatsCLICountIsTheCommandTable: the closing CLI line counts the commands nh
// registers (the retired ones exit 2 and the REPL is retired), never a fixed number.
func TestStatsCLICountIsTheCommandTable(t *testing.T) {
	var out bytes.Buffer
	if err := (&Env{BaseDir: statsRoot(t, ""), Out: &out}).Stats(false); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimRight(out.String(), "\n"), "\n")
	want := fmt.Sprintf("CLI: %d commands", len(commandNames()))
	if got := lines[len(lines)-1]; got != want || retired["repl"] == strings.Contains(out.String(), "REPL") {
		t.Errorf("last line %q, want %q (REPL claimed: %v, retired: %v)", got, want, strings.Contains(out.String(), "REPL"), retired["repl"])
	}
}
