package indexes

import (
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/angeles725/niagara-go/internal/helpbuild/internal/testkit"
)

// The bajadoc_parser.py port on the synthetic javadoc tree of
// testdata/bajadoc/root/ (classes, an interface with subinterfaces, an
// enum, an annotation type, a generic class name, the <title> fallback,
// skipped pages, class-use/ folders, a missing package, CRLF and bytes that
// are not UTF-8, a description wrapped at 78 code points, member rows whose
// fragments feed() holds back). The golden output (testdata/bajadoc/clean/,
// stdout.txt) is Python's, frozen when D47 retired the Python reference;
// TestBajadocCornersFrozen covers what the fixture cannot hold.

// bajadocDone is the elapsed-time line, the one volatile line of the log.
var bajadocDone = regexp.MustCompile(`^Done in \d+\.\ds$`)

// bajadocLog is res.Log with base printed as <base>, the seconds as <t>
// and the host separator as "/": stdout.txt is the POSIX run's, and on
// Windows the Output line carries ntpath.join's backslash, as CPython's
// does there.
func bajadocLog(log []string, base string) []string {
	out := make([]string, len(log))
	for i, l := range log {
		l = strings.ReplaceAll(l, base, "<base>")
		l = strings.ReplaceAll(l, string(filepath.Separator), "/")
		out[i] = bajadocDone.ReplaceAllString(l, "Done in <t>s")
	}
	return out
}

func TestParseBajadocFixture(t *testing.T) {
	base := t.TempDir()
	testkit.CopyTree(t, filepath.Join("testdata", "bajadoc", "root"), base)
	res, err := ParseBajadoc(base)
	if err != nil {
		t.Fatal(err)
	}
	want, err := os.ReadFile(filepath.Join("testdata", "bajadoc", "stdout.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(bajadocLog(res.Log, base), "\n") + "\n"; got != string(want) {
		t.Errorf("log:\n%s\nwant:\n%s", got, want)
	}
	if res.Status != 0 || res.Parsed != 12 || res.Skipped != 3 || res.Errors != 0 {
		t.Errorf("status %d, parsed %d, skipped %d, errors %d; want 0, 12, 3, 0", res.Status, res.Parsed, res.Skipped, res.Errors)
	}
	got, golden := testkit.FileTexts(t, filepath.Join(base, "bajadoc-clean")), testkit.FileTexts(t, filepath.Join("testdata", "bajadoc", "clean"))
	for rel, text := range golden {
		if got[rel] != text {
			t.Errorf("%s:\n got %q\nwant %q", rel, got[rel], text)
		}
	}
	for rel := range got {
		if _, ok := golden[rel]; !ok {
			t.Errorf("%s written, not in the golden tree", rel)
		}
	}
}

// write_clean_class removes the page an older run left under the escaped
// class name ("Cursor&lt;E&gt;.txt") once the clean one is written, and
// leaves the other files of bajadoc-clean/ (bajadoc_xml_parser.py's) alone.
func TestParseBajadocRemovesLegacyName(t *testing.T) {
	base := t.TempDir()
	testkit.CopyTree(t, filepath.Join("testdata", "bajadoc", "root"), base)
	util := filepath.Join(base, "bajadoc-clean", "javax", "baja", "util")
	legacy := filepath.Join(util, "Cursor&lt;E&gt;.txt")
	other := filepath.Join(util, "BXmlOnly.txt")
	testkit.WriteFile(t, legacy, []byte("CLASS: Cursor&lt;E&gt;\n"))
	testkit.WriteFile(t, other, []byte("CLASS: BXmlOnly\n"))
	res, err := ParseBajadoc(base)
	if err != nil || res.Status != 0 || res.Parsed != 12 {
		t.Fatalf("ParseBajadoc: %v, %+v", err, res)
	}
	if _, err := os.Stat(legacy); !os.IsNotExist(err) {
		t.Errorf("escaped-name page kept: %v", err)
	}
	if _, err := os.Stat(filepath.Join(util, "Cursor.txt")); err != nil {
		t.Errorf("clean-name page: %v", err)
	}
	if data, err := os.ReadFile(other); err != nil || string(data) != "CLASS: BXmlOnly\n" {
		t.Errorf("unrelated page: %q, %v; want it untouched", data, err)
	}
}

// Without bajadoc/javax/baja the run prints one ERROR line and exits 1.
func TestParseBajadocMissingDir(t *testing.T) {
	base := t.TempDir()
	res, err := ParseBajadoc(base)
	if err != nil {
		t.Fatal(err)
	}
	want := "ERROR: bajadoc directory not found: " + filepath.Join(base, "bajadoc", "javax", "baja")
	if res.Status != 1 || strings.Join(res.Log, "|") != want {
		t.Errorf("status %d, log %q; want 1, %q", res.Status, res.Log, want)
	}
	if _, err := os.Stat(filepath.Join(base, "bajadoc-clean")); !os.IsNotExist(err) {
		t.Errorf("bajadoc-clean created: %v", err)
	}
}

// A class name that would put its page outside bajadoc-clean/ (Python
// writes it there) stops the run with errUnsafeClassName before any write.
func TestParseBajadocUnsafeClassName(t *testing.T) {
	for _, name := range []string{"../../../../Evil", "/abs/Evil", "x/../../../../../Evil"} {
		base := t.TempDir()
		page := `<div class="subTitle">javax.baja.x</div><h2 class="title">Class ` + name + `</h2>`
		testkit.WriteFile(t, filepath.Join(base, "bajadoc", "javax", "baja", "x", "Evil.html"), []byte(page))
		res, err := ParseBajadoc(base)
		if !errors.Is(err, errUnsafeClassName) || res == nil || res.Parsed != 0 {
			t.Errorf("%q: %v, %+v; want errUnsafeClassName", name, err, res)
		}
		if _, err := os.Stat(filepath.Join(base, "Evil.txt")); !os.IsNotExist(err) {
			t.Errorf("%q: written outside bajadoc-clean: %v", name, err)
		}
	}
}

// format_output wraps the description at 78 code points: a word that would
// pass column 78 starts a line, a longer word stands alone.
func TestFormatBajadocWrap(t *testing.T) {
	c := &bajadocClass{Name: "BX", Kind: "annotation_type", Package: "javax.baja.x",
		Description: strings.Repeat("é", 70) + " abcdefg " + strings.Repeat("y", 80) + " z"}
	want := "ANNOTATION TYPE: BX\nPACKAGE: javax.baja.x\n\nDESCRIPTION:\n" + strings.Repeat("é", 70) +
		" abcdefg\n" + strings.Repeat("y", 80) + "\nz\n"
	if got := formatBajadoc(c); got != want {
		t.Errorf("formatBajadoc:\n got %q\nwant %q", got, want)
	}
}
