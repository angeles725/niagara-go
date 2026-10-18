package helpbuild

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/angeles725/niagara-go/internal/pycompat"
)

// test_extraction.TestExtractAndFidelity.setUp.
func extractFixture(t *testing.T) (install, root string) {
	t.Helper()
	dir := t.TempDir()
	install = filepath.Join(dir, "install")
	writeJar(t, filepath.Join(install, "modules", "docFoo-doc.jar"), [2]string{"META-INF/MANIFEST.MF", "m"},
		[2]string{"doc/", ""}, [2]string{"doc/a.html", "<html>a</html>"}, [2]string{"doc/sub/b.html", "<html>b</html>"},
		[2]string{"doc/crlf.txt", "a\r\nb\r\n"})
	writeJar(t, filepath.Join(install, "modules", "alarm-rt.jar"), [2]string{"x.class", "x"})
	return install, filepath.Join(dir, "root")
}

// test_extract_then_fidelity_is_exact.
func TestExtractThenFidelityIsExact(t *testing.T) {
	install, root := extractFixture(t)
	st, err := Extract(install, root, nil)
	if err != nil || st.Files != 3 {
		t.Fatalf("extract %+v %v", st, err)
	}
	if !pycompat.IsFile(filepath.Join(root, "guides", "Foo", "sub", "b.html")) {
		t.Fatal("guides/Foo/sub/b.html not extracted")
	}
	rep, err := Fidelity(install, root, nil)
	if err != nil || rep.Checked != 3 || rep.Matched != 3 || len(rep.Mismatched)+len(rep.Missing) != 0 || rep.Jars != 1 {
		t.Fatalf("fidelity %+v %v", rep, err)
	}
}

// test_fidelity_detects_changed_and_missing,
// test_fidelity_separates_eol_only_differences.
func TestFidelityClassifiesChangedEOLOnlyAndMissing(t *testing.T) {
	install, root := extractFixture(t)
	if _, err := Extract(install, root, nil); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(root, "guides", "Foo", "a.html"), []byte("tampered"))
	writeFile(t, filepath.Join(root, "guides", "Foo", "crlf.txt"), []byte("a\nb\n"))
	if err := os.Remove(filepath.Join(root, "guides", "Foo", "sub", "b.html")); err != nil {
		t.Fatal(err)
	}
	rep, err := Fidelity(install, root, nil)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(rep.Mismatched, ",") != "guides/Foo/a.html" || strings.Join(rep.Missing, ",") != "guides/Foo/sub/b.html" ||
		strings.Join(rep.EOLOnly, ",") != "guides/Foo/crlf.txt" {
		t.Fatalf("%+v", rep)
	}
}

// test_doc_jar_completeness.TestFillMissing: only absent files are
// written, an edited one is kept, no temp file is left.
func TestFillMissingWritesOnlyAbsentFiles(t *testing.T) {
	install, root := extractFixture(t)
	if _, err := Extract(install, root, nil); err != nil {
		t.Fatal(err)
	}
	kept := filepath.Join(root, "guides", "Foo", "a.html")
	writeFile(t, kept, []byte("locally edited"))
	gone := filepath.Join(root, "guides", "Foo", "sub", "b.html")
	if err := os.Remove(gone); err != nil {
		t.Fatal(err)
	}
	written, err := FillMissing(install, root, nil)
	if err != nil || strings.Join(written, ",") != "guides/Foo/sub/b.html" {
		t.Fatalf("written %q %v", written, err)
	}
	noDotEntries(t, filepath.Dir(gone))
	if readText(t, kept) != "locally edited" || readText(t, gone) != "<html>b</html>" {
		t.Fatal("fill-missing overwrote or did not write")
	}
}

// test_third_party_source_archive_is_excluded_not_extracted and
// test_module_help_jar_and_examples_are_counted: a module jar with doc/
// pages is a help jar, its source archive is neither extracted nor
// missing; a jar with only classes and a stylesheet is not a help jar.
func TestHelpJarsAndSourceArchive(t *testing.T) {
	dir := t.TempDir()
	install := filepath.Join(dir, "install")
	writeJar(t, filepath.Join(install, "modules", "ascHelp.jar"), [2]string{"doc/About-Asc.html", "<p/>"},
		[2]string{"doc/libfoo-1.0.tar.xz", "\xfd7zXZ"}, [2]string{"com/acme/BAsc.class", "\xca\xfe"})
	writeJar(t, filepath.Join(install, "modules", "plainModule-rt.jar"), [2]string{"com/acme/BPlain.class", "\xca\xfe"},
		[2]string{"doc/style.css", "p {}"})
	help, err := HelpJars(install, nil, mustDefault(t))
	if err != nil || len(help) != 1 || filepath.Base(help[0]) != "ascHelp.jar" {
		t.Fatalf("help jars %q %v", help, err)
	}
	root := filepath.Join(dir, "root")
	if _, err := Extract(install, root, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "guides", "ascHelp", "libfoo-1.0.tar.xz")); !os.IsNotExist(err) {
		t.Fatalf("source archive extracted: %v", err)
	}
	rep, err := Fidelity(install, root, nil)
	if err != nil || rep.Checked != 1 || len(rep.Missing) != 0 {
		t.Fatalf("%+v %v", rep, err)
	}
}

// A ".." segment is refused for writes (Python would write outside root);
// nothing is written outside.
func TestExtractRefusesDotDotEntries(t *testing.T) {
	dir := t.TempDir()
	install := filepath.Join(dir, "install")
	writeJar(t, filepath.Join(install, "modules", "docEvil-doc.jar"), [2]string{"doc/../../../escaped.txt", "x"})
	root := filepath.Join(dir, "a", "root")
	if _, err := Extract(install, root, nil); !errors.Is(err, ErrUnsafeEntry) {
		t.Fatalf("extract: %v", err)
	}
	if _, err := FillMissing(install, root, nil); !errors.Is(err, ErrUnsafeEntry) {
		t.Fatalf("fill: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "escaped.txt")); !os.IsNotExist(err) {
		t.Fatal("written outside the root")
	}
	// The Windows spelling is refused on Linux too, where Python would
	// write a file named "..\..\..\escaped.txt" inside the guide folder.
	install = filepath.Join(dir, "install-bs")
	writeJar(t, filepath.Join(install, "modules", "docEvil-doc.jar"), [2]string{`doc/..\..\..\escaped.txt`, "x"})
	if _, err := Extract(install, root, nil); !errors.Is(err, ErrUnsafeEntry) {
		t.Fatalf("extract backslash: %v", err)
	}
	if _, err := FillMissing(install, root, nil); !errors.Is(err, ErrUnsafeEntry) {
		t.Fatalf("fill backslash: %v", err)
	}
}

// outPath refuses, on every OS, a ".." segment split on "/" or "\", a
// leading separator and a drive letter: on Windows filepath.Join treats
// "\" as a separator, so each of them would leave the root there.
func TestOutPathRefusesUnsafeEntries(t *testing.T) {
	root := filepath.Join(t.TempDir(), "root")
	for _, rel := range []string{"../x", "a/../../x", "a/..", "..", `..\x`, `a\..\..\x`, `a\..`, "/abs", `\abs`,
		`\\server\share\x`, `C:\x`, "C:x", "c:/x", "Z:"} {
		if out, err := outPath(root, rel); !errors.Is(err, ErrUnsafeEntry) {
			t.Errorf("%q: %q %v, want ErrUnsafeEntry", rel, out, err)
		}
	}
	// Names that only look odd stay under the root.
	for _, rel := range []string{"a/b.txt", "a/..b", "a/b..", "a/.../x", "a/C:x", "a/b:c"} {
		if out, err := outPath(root, rel); err != nil || out != pycompat.Join(root, rel) {
			t.Errorf("%q: %q %v", rel, out, err)
		}
	}
	// A "\" without "..": Python on Linux writes one file named a\b.txt
	// (kept); on Windows it is the folder a and the file b.txt.
	out, err := outPath(root, `a\b.txt`)
	want := root + `/a\b.txt`
	if runtime.GOOS == "windows" {
		want = filepath.Join(root, "a", "b.txt")
	}
	if err != nil || out != want {
		t.Fatalf("a\\b.txt: %q %v, want %q", out, err, want)
	}
}

// An unreadable doc jar stops extraction (Python raises BadZipFile); an
// unreadable module jar only has no entries.
func TestUnreadableJars(t *testing.T) {
	dir := t.TempDir()
	install := filepath.Join(dir, "install")
	writeFile(t, filepath.Join(install, "modules", "broken-rt.jar"), []byte("not a zip"))
	if _, err := Extract(install, filepath.Join(dir, "r1"), nil); err != nil {
		t.Fatalf("a broken module jar: %v", err)
	}
	writeFile(t, filepath.Join(install, "modules", "docBroken-doc.jar"), []byte("not a zip"))
	if _, err := Extract(install, filepath.Join(dir, "r2"), nil); !errors.Is(err, errBadZip) {
		t.Fatalf("a broken doc jar: %v", err)
	}
}
