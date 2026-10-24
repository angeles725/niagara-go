package help

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/angeles725/niagara-go/internal/help/fts"
	"github.com/angeles725/niagara-go/internal/help/fts/ftsfixture"
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

INDEXES (12):

  class-index.json            0 classes (0 packages)
  devguide-toc.json           0 guides
  guides-index.json           0 folders
  inheritance.json            0 entries
  pdf-index.json              0 PDFs
  method-index.json           0 names (0 entries)
  source-index.json           0 classes (0 lines)
  xref-index.json             0 classes (0 edges)
  slots-index.json            0 classes (0 props, 0 actions, 0 topics)
  module-index.json      absent (rebuild-all builds it from the install's modules/)
  palette-index.json     absent (rebuild-all builds it from the install's modules/)
  tag-index.json         absent (rebuild-all builds it from the install's modules/)

TEXT SOURCES (10 sources of the full-text index, 0.0 MB total):

  bajadoc-clean/      absent
  devguide-clean/     absent
  guides-clean/       absent
  docs-text/          absent
  course-text/        absent
  source/             absent
  devguide-rc/        absent
  devguide-examples/  absent
  package-text/       absent
  lexicon-text/       absent

RAW HTML:

  bajadoc/                 0 HTML
  devguide/                0 HTML
  guides/                  0 HTML

INDEXES: 12
CLI: 24 commands
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

// TestStatsListsEveryFulltextSource: TEXT SOURCES lists every source of the full-text
// index (fts.Labels, the one source registry), counts only the files the index reads, says
// "absent" for a source not built, and names the version a borrowed source comes from.
func TestStatsListsEveryFulltextSource(t *testing.T) {
	dir := statsRoot(t, "")
	for rel, body := range map[string]string{
		"lexicon-text/kitControl/a.txt": "abc", "package-text/javax.baja.sys.txt": "de",
		"course-text/day1.txt": "f", "devguide-examples/x/a.js": "g", "devguide-examples/x/notes.md": "skipped",
		"source/a/B.java": "class B {}",
	} {
		p := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	var out bytes.Buffer
	if err := (&Env{BaseDir: dir, Out: &out}).Stats(false); err != nil {
		t.Fatal(err)
	}
	got := out.String()
	for _, label := range fts.Labels {
		if !strings.Contains(got, "\n  "+label+"/ ") {
			t.Errorf("source %s not listed:\n%s", label, got)
		}
	}
	for _, want := range []string{
		"TEXT SOURCES (10 sources of the full-text index, 0.0 MB total):",
		"  lexicon-text/            1 files  (0.0 MB)",
		"  devguide-examples/       1 files  (0.0 MB)",
		"  docs-text/          absent",
	} {
		if !strings.Contains(got, want+"\n") {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}
}

// TestStatsNamesASharedSource: a version without its own PDF manuals says the docs-text/
// it reports is the shared one of another version, as find and pdf do.
func TestStatsNamesASharedSource(t *testing.T) {
	testdata, err := filepath.Abs("testdata")
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(ftsfixture.Checkout(t, testdata, true), "versions", "2.0")
	for _, n := range statsIndexNames {
		p := filepath.Join(root, "indexes", n+".json")
		if _, err := os.Stat(p); err == nil {
			continue
		}
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("{}"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	var out bytes.Buffer
	if err := (&Env{BaseDir: root, Out: &out}).Stats(false); err != nil {
		t.Fatal(err)
	}
	if want := "files  (0.0 MB)  shared: the PDF manuals of Niagara 1.0 (this version ships none)\n"; !strings.Contains(out.String(), want) {
		t.Errorf("no shared docs-text note %q in:\n%s", want, out.String())
	}
}

// TestStatsCountsModuleIndex: module-index.json is optional (built from the
// install, not the data root); when present stats counts its modules and
// elements and the descriptors it could not read, in text and JSON.
func TestStatsCountsModuleIndex(t *testing.T) {
	dir := statsRoot(t, "")
	data, err := os.ReadFile(filepath.Join("testdata", "indexes", "module-index.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "indexes", "module-index.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := (&Env{BaseDir: dir, Out: &out}).Stats(false); err != nil {
		t.Fatal(err)
	}
	want := "  module-index.json           5 modules (6 dependencies, 6 types, 2 agents, 2 permissions; 1 unreadable)\n"
	if !strings.Contains(out.String(), want) {
		t.Errorf("stats lacks %q:\n%s", want, out.String())
	}
	out.Reset()
	if err := (&Env{BaseDir: dir, Out: &out}).Stats(true); err != nil {
		t.Fatal(err)
	}
	want = `"module-index": {
      "modules": 5,
      "dependencies": 6,
      "types": 6,
      "agents": 2,
      "java_permissions": 2,
      "unreadable": 1
    }`
	if !strings.Contains(out.String(), want) {
		t.Errorf("stats --json lacks %s:\n%s", want, out.String())
	}
}

// TestStatsCountsPaletteIndex: stats counts palette-index.json's palettes,
// entries and unreadable palettes, in text and JSON.
func TestStatsCountsPaletteIndex(t *testing.T) {
	dir := statsRoot(t, "")
	data, err := os.ReadFile(filepath.Join("testdata", "indexes", "palette-index.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "indexes", "palette-index.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := (&Env{BaseDir: dir, Out: &out}).Stats(false); err != nil {
		t.Fatal(err)
	}
	want := "  palette-index.json          2 palettes (5 entries; 1 unreadable)\n"
	if !strings.Contains(out.String(), want) {
		t.Errorf("stats lacks %q:\n%s", want, out.String())
	}
	out.Reset()
	if err := (&Env{BaseDir: dir, Out: &out}).Stats(true); err != nil {
		t.Fatal(err)
	}
	want = `"palette-index": {
      "palettes": 2,
      "entries": 5,
      "unreadable": 1
    }`
	if !strings.Contains(out.String(), want) {
		t.Errorf("stats --json lacks %s:\n%s", want, out.String())
	}
}

// TestStatsCountsTagIndex: stats counts tag-index.json's distinct tags, its
// Haystack 4 defs and every record section, the files read and those it
// could not read, in text and JSON.
func TestStatsCountsTagIndex(t *testing.T) {
	dir := statsRoot(t, "")
	data, err := os.ReadFile(filepath.Join("testdata", "indexes", "tag-index.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "indexes", "tag-index.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := (&Env{BaseDir: dir, Out: &out}).Stats(false); err != nil {
		t.Fatal(err)
	}
	want := "  tag-index.json              3 tags (2 defs, 2 protos, 1 groups, 1 relations, 1 rules, 1 migrations; 7 files, 1 unreadable)\n"
	if !strings.Contains(out.String(), want) {
		t.Errorf("stats lacks %q:\n%s", want, out.String())
	}
	out.Reset()
	if err := (&Env{BaseDir: dir, Out: &out}).Stats(true); err != nil {
		t.Fatal(err)
	}
	want = `"tag-index": {
      "tags": 3,
      "defs": 2,
      "protos": 2,
      "groups": 1,
      "relations": 1,
      "rules": 1,
      "migrations": 1,
      "files": 7,
      "unreadable": 1
    }`
	if !strings.Contains(out.String(), want) {
		t.Errorf("stats --json lacks %s:\n%s", want, out.String())
	}
}
