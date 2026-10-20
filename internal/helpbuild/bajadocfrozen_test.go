package helpbuild

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/angeles725/niagara-go/internal/helpbuild/internal/testkit"
)

// The S7a bajadoc parser (bajadoc/javax/baja/**.html -> bajadoc-clean/)
// and the S7b builder (.bajadoc XML -> bajadoc-clean/) on the corners a
// checked-in fixture cannot hold, against their frozen Python output
// (frozen_test.go): the printed lines (paths shown as <base>, the "Done in"
// seconds as <t>), the exit status and the whole tree after each run.

// goBajadocRun is ParseBajadoc as lines: ["out", line] per log line, then
// ["exit", status] or ["raise"].
func goBajadocRun(base string) ([]string, time.Duration) {
	start := time.Now()
	res, err := ParseBajadoc(base)
	took := time.Since(start)
	var out []string
	if res != nil {
		for _, l := range res.Log {
			out = append(out, testkit.DumpLine(testkit.Str("out"), testkit.Str(l)))
		}
	}
	if err != nil {
		return append(out, testkit.DumpLine(testkit.Str("raise"))), took
	}
	return append(out, testkit.DumpLine(testkit.Str("exit"), testkit.IntV(res.Status))), took
}

// goBajadocXMLRun is ParseBajadocXML as lines, like goBajadocRun.
func goBajadocXMLRun(base string) ([]string, time.Duration) {
	start := time.Now()
	res, err := ParseBajadocXML(base)
	took := time.Since(start)
	var out []string
	for _, l := range res.Log {
		out = append(out, testkit.DumpLine(testkit.Str("out"), testkit.Str(l)))
	}
	if err != nil {
		return append(out, testkit.DumpLine(testkit.Str("raise"))), took
	}
	return append(out, testkit.DumpLine(testkit.Str("exit"), testkit.IntV(0))), took
}

// relabelBajadoc shows base as <base> and the elapsed seconds as <t>.
func relabelBajadoc(lines []string, base string) []string {
	out := make([]string, len(lines))
	for i, l := range lines {
		if strings.HasPrefix(l, `["out", "Done in `) {
			l = testkit.DumpLine(testkit.Str("out"), testkit.Str("Done in <t>s"))
		}
		out[i] = strings.ReplaceAll(l, base, "<base>")
	}
	return out
}

// bajadocFrozen runs run on base and checks its lines and, after each of
// after (unlocking pages the tree listing must read), the whole tree
// against testdata/frozen/<name>.txt.
func bajadocFrozen(t *testing.T, name, base string, run func(string) ([]string, time.Duration), after ...func()) {
	t.Helper()
	lines, _ := run(base)
	for _, f := range after {
		f()
	}
	got := append(relabelBajadoc(lines, base), "tree:")
	testkit.CheckFrozen(t, name, append(got, testkit.TreeLines(t, base)...))
}

// bajadocCorners adds what a checked-in fixture cannot hold: an unreadable
// page (unlock makes it readable again), a dangling link, a link to a page
// and one to a folder, a page left under its escaped name and an unrelated
// page bajadoc_xml_parser wrote.
func bajadocCorners(t *testing.T, base string) (unlock func()) {
	t.Helper()
	util := filepath.Join(base, "bajadoc", "javax", "baja", "util")
	locked := filepath.Join(util, "Locked.html")
	testkit.WriteFile(t, locked, []byte(`<h2 class="title">Class Locked</h2>`))
	if err := os.Chmod(locked, 0); err != nil {
		t.Fatal(err)
	}
	unlock = func() { _ = os.Chmod(locked, 0o644) }
	t.Cleanup(unlock)
	for link, target := range map[string]string{"Dangling.html": "missing.html", "Alias.html": "Cursor.html",
		"linked": "../sys"} {
		if err := os.Symlink(target, filepath.Join(util, link)); err != nil {
			t.Fatal(err)
		}
	}
	clean := filepath.Join(base, "bajadoc-clean", "javax", "baja")
	testkit.WriteFile(t, filepath.Join(clean, "util", "Cursor&lt;E&gt;.txt"), []byte("old escaped name"))
	testkit.WriteFile(t, filepath.Join(clean, "xml", "BFromXml.txt"), []byte("CLASS: BFromXml\n"))
	return unlock
}

// TestBajadocCornersFrozen: the checked-in fixture plus the corners, twice
// (the second run overwrites in place), and a data root without bajadoc/
// (exit 1). The fixture's own output is checked by TestParseBajadocFixture.
func TestBajadocCornersFrozen(t *testing.T) {
	testkit.FrozenOnly(t)
	if os.Geteuid() == 0 {
		t.Skip("root reads a mode-000 page")
	}
	base := filepath.Join(t.TempDir(), "go")
	testkit.CopyTree(t, filepath.Join("testdata", "bajadoc", "root"), base)
	goBajadocRun(base)
	unlock := bajadocCorners(t, base)
	bajadocFrozen(t, "bajadoc/corners", base, goBajadocRun, unlock)
	bajadocFrozen(t, "bajadoc/second-run", base, goBajadocRun)
	empty := filepath.Join(t.TempDir(), "go")
	testkit.WriteFile(t, filepath.Join(empty, "bajadoc", "javax", "readme.txt"), nil)
	bajadocFrozen(t, "bajadoc/no-javax-baja", empty, goBajadocRun)
}

// bajadocXMLCorners adds what a checked-in fixture cannot hold: a page left
// under the escaped generic name, S7a's page of a class with a javadoc
// page, a link to a .bajadoc file (read) and one to a folder (not entered).
func bajadocXMLCorners(t *testing.T, base string) {
	t.Helper()
	demo := filepath.Join(base, "bajadoc-clean", "javax", "baja", "demo")
	if runtime.GOOS != "windows" {
		testkit.WriteFile(t, filepath.Join(demo, "Box<T>.txt"), []byte("old escaped name"))
	}
	testkit.WriteFile(t, filepath.Join(demo, "BHasHtml.txt"), []byte("CLASS: BHasHtml\nfrom the javadoc page\n"))
	testkit.WriteFile(t, filepath.Join(base, "extra", "BLinked.bajadoc"),
		[]byte(`<bajadoc><class qualifiedName="a.BLinked" name="BLinked" packageName="a"/></bajadoc>`))
	testkit.WriteFile(t, filepath.Join(base, "extra", "dir", "BNotEntered.bajadoc"),
		[]byte(`<bajadoc><class qualifiedName="a.BNotEntered" name="BNotEntered" packageName="a"/></bajadoc>`))
	guides := filepath.Join(base, "guides")
	for link, target := range map[string]string{"Linked.bajadoc": "../extra/BLinked.bajadoc", "linked-dir": "../extra/dir"} {
		if err := os.Symlink(target, filepath.Join(guides, link)); err != nil {
			t.Fatal(err)
		}
	}
}

// TestBajadocXMLCornersFrozen: the checked-in fixture, the corners, a
// second run over the first one's pages, a .bajadoc file that cannot be
// read (dangling link, mode 000: both raised) and a data root without
// devguide/ or guides/. The fixture's own output is checked by
// TestParseBajadocXMLFixture.
func TestBajadocXMLCornersFrozen(t *testing.T) {
	testkit.FrozenOnly(t)
	base := filepath.Join(t.TempDir(), "go")
	testkit.CopyTree(t, filepath.Join("testdata", "bajadocxml", "root"), base)
	goBajadocXMLRun(base)
	bajadocXMLCorners(t, base)
	bajadocFrozen(t, "bajadocxml/corners", base, goBajadocXMLRun)
	bajadocFrozen(t, "bajadocxml/second-run", base, goBajadocXMLRun)
	if err := os.Symlink("missing.bajadoc", filepath.Join(base, "devguide", "Dangling.bajadoc")); err != nil {
		t.Fatal(err)
	}
	bajadocFrozen(t, "bajadocxml/dangling-link", base, goBajadocXMLRun)
	if os.Geteuid() != 0 {
		_ = os.Remove(filepath.Join(base, "devguide", "Dangling.bajadoc"))
		locked := filepath.Join(base, "guides", "vendor", "com", "vendor", "BVendor.bajadoc")
		if err := os.Chmod(locked, 0); err != nil {
			t.Fatal(err)
		}
		restore := func() { _ = os.Chmod(locked, 0o644) }
		t.Cleanup(restore)
		bajadocFrozen(t, "bajadocxml/unreadable", base, goBajadocXMLRun, restore)
	}
	empty := filepath.Join(t.TempDir(), "go")
	testkit.WriteFile(t, filepath.Join(empty, "devguide-not", "x.bajadoc"), []byte(`<bajadoc/>`))
	bajadocFrozen(t, "bajadocxml/no-devguide-or-guides", empty, goBajadocXMLRun)
}
