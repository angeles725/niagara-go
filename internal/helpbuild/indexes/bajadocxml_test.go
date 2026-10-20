package indexes

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/angeles725/niagara-go/internal/helpbuild/internal/testkit"
)

// The bajadoc_xml_parser.py port on the synthetic tree of
// testdata/bajadocxml/root/: classes in devguide/ and guides/ (a class, an
// enum, an annotation type, an interface, a generic name, no package), a
// class with a javadoc page (skipped), a class in both trees and one in two
// devguide modules (the first walked wins), an index file, files that do
// not parse even sanitized, a forbidden character reference, CRLF, files
// that are not .bajadoc, and the SUBCLASSES lines derived from extends.
// The golden output (testdata/bajadocxml/clean/, stdout.txt) is Python's,
// frozen when D47 retired the Python reference; TestBajadocXMLCornersFrozen
// covers what the fixture cannot hold.

func TestParseBajadocXMLFixture(t *testing.T) {
	base := t.TempDir()
	testkit.CopyTree(t, filepath.Join("testdata", "bajadocxml", "root"), base)
	res, err := ParseBajadocXML(base)
	if err != nil {
		t.Fatal(err)
	}
	want, err := os.ReadFile(filepath.Join("testdata", "bajadocxml", "stdout.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(res.Log, "\n") + "\n"; got != string(want) || res.Classes != 12 {
		t.Errorf("log %q, %d classes; want %q, 12", got, res.Classes, want)
	}
	got, golden := testkit.FileTexts(t, filepath.Join(base, "bajadoc-clean")), testkit.FileTexts(t, filepath.Join("testdata", "bajadocxml", "clean"))
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

// The pages of bajadoc_parser.py (classes with a javadoc page) are left
// alone, a page left under the escaped generic name is removed, and a data
// root without devguide/ or guides/ writes nothing.
func TestParseBajadocXMLKeepsOtherPages(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("a file name with < and > cannot exist on Windows")
	}
	base := t.TempDir()
	testkit.CopyTree(t, filepath.Join("testdata", "bajadocxml", "root"), base)
	demo := filepath.Join(base, "bajadoc-clean", "javax", "baja", "demo")
	html, legacy := filepath.Join(demo, "BHasHtml.txt"), filepath.Join(demo, "Box<T>.txt")
	testkit.WriteFile(t, html, []byte("CLASS: BHasHtml\nfrom the javadoc page\n"))
	testkit.WriteFile(t, legacy, []byte("old"))
	if _, err := ParseBajadocXML(base); err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(html); err != nil || string(data) != "CLASS: BHasHtml\nfrom the javadoc page\n" {
		t.Errorf("javadoc page text: %q, %v; want it untouched", data, err)
	}
	if _, err := os.Stat(legacy); !os.IsNotExist(err) {
		t.Errorf("escaped-name page kept: %v", err)
	}

	empty := t.TempDir()
	res, err := ParseBajadocXML(empty)
	if err != nil || res.Classes != 0 || len(res.Log) != 1 {
		t.Fatalf("empty root: %v, %+v", err, res)
	}
	if _, err := os.Stat(filepath.Join(empty, "bajadoc-clean")); !os.IsNotExist(err) {
		t.Errorf("bajadoc-clean created for no class: %v", err)
	}
}

// A .bajadoc file that cannot be read stops the run before any write, as
// ET.parse raises the OSError (only a ParseError is caught).
func TestParseBajadocXMLReadError(t *testing.T) {
	base := t.TempDir()
	testkit.CopyTree(t, filepath.Join("testdata", "bajadocxml", "root"), base)
	if err := os.Symlink("missing.bajadoc", filepath.Join(base, "guides", "Dangling.bajadoc")); err != nil {
		t.Skipf("no symlinks here: %v", err)
	}
	res, err := ParseBajadocXML(base)
	if !errors.Is(err, os.ErrNotExist) || len(res.Log) != 0 {
		t.Errorf("ParseBajadocXML: %v, %+v; want a not-exist error and no line", err, res)
	}
	if _, err := os.Stat(filepath.Join(base, "bajadoc-clean")); !os.IsNotExist(err) {
		t.Errorf("bajadoc-clean written before the error: %v", err)
	}
}

// A class name that would put its page outside bajadoc-clean/ stops the
// run with ErrUnsafeClassName (Python writes it there).
func TestParseBajadocXMLUnsafeClassName(t *testing.T) {
	base := t.TempDir()
	doc := `<bajadoc><class qualifiedName="x.Evil" name="../../../../Evil" packageName="x"/></bajadoc>`
	testkit.WriteFile(t, filepath.Join(base, "devguide", "Evil.bajadoc"), []byte(doc))
	if _, err := ParseBajadocXML(base); !errors.Is(err, errUnsafeClassName) {
		t.Errorf("ParseBajadocXML: %v; want ErrUnsafeClassName", err)
	}
	if _, err := os.Stat(filepath.Join(base, "Evil.txt")); !os.IsNotExist(err) {
		t.Errorf("written outside bajadoc-clean: %v", err)
	}
}
