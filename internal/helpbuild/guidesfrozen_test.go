package helpbuild

import (
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/angeles725/niagara-go/internal/helpbuild/internal/testkit"
	"github.com/angeles725/niagara-go/internal/pyjson"
)

// The S5 parsers (guides-clean/, devguide-clean/) and the S6 indexes
// (guides-index.json, devguide-toc.json) against their frozen Python output
// (frozen_test.go): the printed lines of each builder and how the tree
// changed, and parse_toc / count_items on the toc corners.

// goRun runs one builder as lines: ["out", line] and ["err", warning] per
// line, then ["result", files] (null for the indexes) or ["raise"].
func goRun(builder, base string) []string {
	var res *TextResult
	var err error
	result := pyjson.Value{}
	switch builder {
	case "guides":
		res, err = ParseGuides(base)
	case "devguide":
		res, err = ParseDevguide(base)
	case "guidesindex":
		res, err = BuildGuidesIndex(base)
	case "devguidetoc":
		res, err = BuildDevguideTOC(base)
	}
	if err != nil {
		return []string{testkit.DumpLine(testkit.Str("raise"))}
	}
	var out []string
	for _, l := range res.Log {
		out = append(out, testkit.DumpLine(testkit.Str("out"), testkit.Str(l)))
	}
	for _, l := range res.Warnings {
		out = append(out, testkit.DumpLine(testkit.Str("err"), testkit.Str(l)))
	}
	if builder == "guides" || builder == "devguide" {
		result = testkit.IntV(res.Files)
	}
	return append(out, testkit.DumpLine(testkit.Str("result"), result))
}

// relabelRun replaces base in the lines by <base> and keeps only the
// exception class position of a raise (Go reports an error, not a class).
func relabelRun(lines []string, base string) []string {
	out := make([]string, len(lines))
	for i, l := range lines {
		if strings.HasPrefix(l, `["raise"`) {
			l = `["raise"]`
		}
		out[i] = strings.ReplaceAll(l, base, "<base>")
	}
	return out
}

// guidesFrozen runs builders in order on base; each step checks the lines
// of the builder and the tree delta it made against
// testdata/frozen/guides/<name>/<n>-<builder>.txt.
func guidesFrozen(t *testing.T, name, base string, builders ...string) {
	t.Helper()
	prev := testkit.TreeLines(t, base)
	for i, builder := range builders {
		lines := append(relabelRun(goRun(builder, base), base), "tree:")
		cur := testkit.TreeLines(t, base)
		testkit.CheckFrozen(t, fmt.Sprintf("guides/%s/%d-%s", name, i+1, builder), append(lines, testkit.TreeDelta(prev, cur)...))
		prev = cur
	}
}

const tocLatin1 = "<?xml version=\"1.0\" encoding=\"ISO-8859-1\" standalone=\"no\"?>\n" +
	"<!DOCTYPE toc\n  PUBLIC \"-//Sun Microsystems Inc.//DTD JavaHelp TOC Version 1.0//EN\" " +
	"\"http://java.sun.com/products/javahelp/toc_1_0.dtd\">\n<toc version=\"1.0\">\n" +
	"  <tocitem target=\"index.html\" text=\"Spy  Guide\"/>\n" +
	"  <tocitem target=\"Tasks.html#a\" text=\"Using \xe9t\xe9 &amp; &#8220;quotes&#8221;\">\n" +
	"    <tocitem target=\"sub\\Deep.html\" text=\"Deep&#xA;page\"/>\n" +
	"    <tocitem target=\"sub/Deep.html\" text=\"Second title\"/>\n" +
	"    <tocitem text=\"No target\"><tocitem target=\"notes.md\" text=\"Notes\"/></tocitem>\n" +
	"    <tocitem target=\"Plain.txt\" text=\"\"/>\n" +
	"    <tocitem target=\" Plain.txt \" text=\"Plain text\"/>\n" +
	"    <tocitem target=\"missing.html\" text=\"Not on disk\"/>\n" +
	"  </tocitem>\n</toc>\n"

// guidesFixture writes the synthetic corpus root: the test_extraction.py
// TestRecursiveGuideExtraction pages plus toc.xml corners, plain-text and
// Markdown pages, a licence text, an unreadable-as-UTF-8 page, CRLF, a
// broken toc, an upper-case TOC.XML, and a devguide/ with an index.html
// for devguide-toc.
func guidesFixture(t *testing.T, base string) {
	t.Helper()
	page := func(title, body string) string {
		return "<html><head><title>" + title + "</title></head><body><p>" + body + "</p></body></html>"
	}
	files := map[string]string{
		"guides/Spy/top.html":                  page("Top Page", "top body"),
		"guides/Spy/A/Compile.html":            page("Compile A", "alpha body"),
		"guides/Spy/B/Compile.html":            page("Compile B", "beta body"),
		"guides/Spy/B/Deep/legacy.htm":         page("Legacy Htm", "gamma body"),
		"guides/Spy/top.htm":                   page("Top Htm", "htm twin body"),
		"guides/Spy/stub.html":                 "<html><head><title></title></head><body></body></html>",
		"guides/Toc/toc.xml":                   tocLatin1,
		"guides/Toc/index.html":                page("Spy Guide", "index"),
		"guides/Toc/Tasks.html":                page("Tasks", "tasks\r\nbody"),
		"guides/Toc/sub/Deep.html":             "<h1>Deep</h1><pre>  a\r\n  b</pre>",
		"guides/Toc/notes.md":                  "#   ###   \n# C# notes ##\r\nbody\n",
		"guides/Toc/readme.MD":                 "no heading\n",
		"guides/Toc/Plain.txt":                 "\n  plain text \r\n",
		"guides/Toc/Plain.html":                page("Plain", "html twin"),
		"guides/Toc/LICENSE.txt":               "licence",
		"guides/Toc/lgpl-2.1.htm":              page("LGPL", "kept: only .txt licences are skipped"),
		"guides/Toc/bad.html":                  "<p>caf\xe9 \xed\xa0\x80</p>",
		"guides/Toc/Empty.txt":                 "   \n",
		"guides/Bad/toc.xml":                   "<toc><tocitem></toc>",
		"guides/Bad/a.html":                    page("A", "a"),
		"guides/Upper/TOC.XML":                 "<toc><tocitem target=\"x.html\" text=\"X\"/></toc>",
		"guides/Upper/toc.xml.bak":             "",
		"guides/Upper/x.html":                  page("", "<h1>H</h1>"),
		"guides/Upper/y.HTML":                  page("Y", "y"),
		"guides/Ünïcode/a.html":                page("Ün", "日本"),
		"guides/notafolder.html":               page("N", "n"),
		"devguide/index.html":                  devguideIndex,
		"devguide/changes.html":                "<html><title>Changes</title><p>html page words</p>",
		"devguide/changes.txt":                 "plain text page words\n",
		"devguide/lgpl-2.1.txt":                "GNU",
		"devguide/sub/COPYING.txt":             "c",
		"devguide/sub/page.html":               page("Sub", "sub &amp; page"),
		"devguide/sub/skipped.htm":             page("Htm", "not a devguide page"),
		"devguide/sub/deeper/releaseNotes.txt": "notes\r\n",
		"devguide/sub/deeper/empty.html":       "",
	}
	for rel, text := range files {
		testkit.WriteFile(t, filepath.Join(base, filepath.FromSlash(rel)), []byte(text))
	}
	for _, d := range []string{"guides/Empty", "guides/Upper/toc.xml.d"} {
		if err := os.MkdirAll(filepath.Join(base, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
}

// TestGuidesFrozen: every builder on the synthetic corpus, then a second
// run that replaces the clean trees and re-reads the index it wrote.
func TestGuidesFrozen(t *testing.T) {
	testkit.FrozenOnly(t)
	base := filepath.Join(t.TempDir(), "go")
	guidesFixture(t, base)
	guidesFrozen(t, "synthetic", base, "guides", "devguide", "guidesindex", "devguidetoc", "guides", "guidesindex")
}

// TestGuidesNonUTF8NamesFrozen: page and folder names that are not UTF-8
// make both parsers raise (the old clean tree is kept); guides-index.json
// spells them as Python's surrogate escapes, sorted as decoded names.
func TestGuidesNonUTF8NamesFrozen(t *testing.T) {
	testkit.FrozenOnly(t)
	base := filepath.Join(t.TempDir(), "go")
	for _, rel := range []string{"guides/Odd\xff/a\xfe.html", "guides/Odd\xff/b.html", "guides/Odd\xff/\xee\x80\x80.html",
		"guides/Plain/c.html", "guides/\xee\x80\x80/d.html", "devguide/x\xff.html", "devguide-clean/old.txt",
		"guides-clean/old.txt"} {
		testkit.WriteFile(t, filepath.Join(base, filepath.FromSlash(rel)), []byte("<title>T</title><p>x</p>"))
	}
	guidesFrozen(t, "non-utf8", base, "guides", "devguide", "guidesindex")
}

// registeredFixture lays out repo/versions/<vid>/ as a registered data root
// with a version manifest and an install whose modules/ holds Upper.jar;
// mounted false points install_dir at a folder that does not exist.
func registeredFixture(t *testing.T, repo string, mounted, manifest bool) string {
	t.Helper()
	base := filepath.Join(repo, "versions", "9.9.9")
	guidesFixture(t, base)
	install := filepath.Join(repo, "install")
	if mounted {
		testkit.WriteFile(t, filepath.Join(install, "modules", "Upper.jar"), []byte("x"))
		testkit.WriteFile(t, filepath.Join(install, "modules", "docToc-doc.jar"), []byte("x"))
	}
	reg := map[string]any{"versions": map[string]any{"9.9.9": map[string]any{
		"label": "test", "install_dir": install, "data_root": "versions/9.9.9"}}}
	data, err := json.Marshal(reg)
	if err != nil {
		t.Fatal(err)
	}
	testkit.WriteFile(t, filepath.Join(repo, "versions", "registry.json"), data)
	if manifest {
		testkit.WriteFile(t, filepath.Join(repo, "versions", "9.9.9.manifest.json"),
			[]byte(`{"jars": [{"jar": "modules/docToc-doc.jar"}, {"jar": "modules/docSpy-doc.jar"}, {"jar": "x/docNone-doc.jar"}]}`))
	}
	// An earlier index whose values are kept when they cannot be checked.
	testkit.WriteFile(t, filepath.Join(base, "indexes", "guides-index.json"),
		[]byte(`{"Spy": {"source_jar": "old-spy.jar"}, "Bad": {"source_jar": 0}, "Upper": "not a dict", "Empty": {}}`))
	return base
}

// TestGuidesIndexSourcesFrozen: guides-index.json's source jars from a
// mounted install, an unmounted one and a version without a manifest, over
// an earlier index whose unchecked values are kept.
func TestGuidesIndexSourcesFrozen(t *testing.T) {
	testkit.FrozenOnly(t)
	for _, c := range []struct {
		name              string
		mounted, manifest bool
	}{{"mounted", true, true}, {"unmounted", false, true}, {"no-manifest", true, false}} {
		base := registeredFixture(t, filepath.Join(t.TempDir(), "go"), c.mounted, c.manifest)
		guidesFrozen(t, "sources-"+c.name, base, "guidesindex")
	}
}

// TestTocFilesFrozen: parse_toc and count_items on every toc.xml of the
// synthetic corpus plus the toc corners.
func TestTocFilesFrozen(t *testing.T) {
	testkit.FrozenOnly(t)
	dir := t.TempDir()
	guidesFixture(t, dir)
	corners := map[string]string{
		"ns.xml":           `<toc xmlns="urn:x"><tocitem target="a.html" text="A"/></toc>`,
		"rootitem.xml":     `<tocitem target="r.html" text="Root"><tocitem target="c.html" text="C"/></tocitem>`,
		"empty.xml":        "",
		"undefined.xml":    "<!DOCTYPE toc SYSTEM \"x.dtd\"><toc><tocitem target=\"u.html\" text=\"&nbsp;x\"/></toc>",
		"bom.xml":          "\xef\xbb\xbf<?xml version=\"1.0\" encoding=\"ISO-8859-1\"?><toc><tocitem target=\"b.html\" text=\"\xe9\"/></toc>",
		"junk.xml":         "<toc/><x/>",
		"bomlatin1bad.xml": "\xef\xbb\xbf<?xml version=\"1.0\" encoding=\"ISO-8859-1\"?><toc><tocitem></toc>",
		"bomascii.xml":     "\xef\xbb\xbf<?xml version='1.0' encoding='US-ASCII'?>\n<toc><tocitem target='a.html' text='\xe9'/></toc>",
		"bomutf8.xml":      "\xef\xbb\xbf<?xml version=\"1.0\" encoding=\"utf-8\"?><toc><tocitem target=\"a.html\" text=\"\xc3\xa9\"/><x></toc>",
		"bomutf16.xml":     "\xef\xbb\xbf<?xml version=\"1.0\" encoding=\"UTF-16\"?><toc/>",
		"utf8.xml":         "<?xml version='1.0' encoding='UTF-8'?><toc><tocitem target='u.html' text='\xc3\xa9 \xe2\x80\x94'/></toc>",
	}
	paths := []string{
		filepath.Join(dir, "guides", "Toc", "toc.xml"), filepath.Join(dir, "guides", "Bad", "toc.xml"),
		filepath.Join(dir, "guides", "Upper", "TOC.XML"), filepath.Join(dir, "missing.xml"),
	}
	for _, name := range slices.Sorted(maps.Keys(corners)) {
		p := filepath.Join(dir, name)
		testkit.WriteFile(t, p, []byte(corners[name]))
		paths = append(paths, p)
	}
	var gol []string
	for _, p := range paths {
		toc, warning := ParseToc(p)
		rows := testkit.Array()
		for _, k := range toc.Order {
			e := toc.Pages[k]
			trail := testkit.Array()
			for _, s := range e.Trail {
				trail.A = append(trail.A, testkit.Str(s))
			}
			rows.A = append(rows.A, testkit.Array(testkit.Str(k), testkit.Str(e.Title), trail))
		}
		warnings := testkit.Array()
		if warning != "" {
			warnings.A = append(warnings.A, testkit.Str(warning))
		}
		gol = append(gol, testkit.DumpLine(testkit.Str("toc"), rows, testkit.IntV(CountItems(p)), warnings))
	}
	testkit.CheckFrozen(t, "guides/toc", testkit.Relabeled(gol, dir, "<dir>"))
}
