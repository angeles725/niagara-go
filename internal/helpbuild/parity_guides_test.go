package helpbuild

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/angeles725/niagara-go/internal/pyjson"
)

// Parity of the S5 parsers (guides-clean/, devguide-clean/) and the S6
// indexes (guides-index.json, devguide-toc.json): each side builds into its
// own copy of the input, then the printed lines and both whole trees are
// compared (htmldriver.py run).

// goRun is htmldriver.py run rebuilt from this package.
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
		return []string{dumpLine(str("raise"))}
	}
	var out []string
	for _, l := range res.Log {
		out = append(out, dumpLine(str("out"), str(l)))
	}
	for _, l := range res.Warnings {
		out = append(out, dumpLine(str("err"), str(l)))
	}
	if builder == "guides" || builder == "devguide" {
		result = intV(res.Files)
	}
	return append(out, dumpLine(str("result"), result))
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

// runBoth runs one builder on the Python copy and the Go copy and compares
// the lines and the two trees.
func runBoth(t *testing.T, builder, pyBase, goBase string) {
	t.Helper()
	py := relabelRun(runHTMLDriver(t, "", "run", builder, pyBase), pyBase)
	gol := relabelRun(goRun(builder, goBase), goBase)
	diffLines(t, builder+" lines", py, gol)
	files, size := diffContents(t, pyBase, goBase)
	t.Logf("%s: %d files, %d bytes compared", builder, files, size)
	diffLines(t, builder+" tree", treeLines(t, pyBase), treeLines(t, goBase))
}

// diffContents shows where the first few files that differ do, before
// treeLines reports only their hashes.
func diffContents(t *testing.T, pyBase, goBase string) (files, size int) {
	t.Helper()
	bad := 0
	_ = filepath.WalkDir(pyBase, func(p string, d os.DirEntry, err error) error {
		if err != nil || !d.Type().IsRegular() || bad >= 5 {
			return nil
		}
		rel, _ := filepath.Rel(pyBase, p)
		a, errA := os.ReadFile(p)
		b, errB := os.ReadFile(filepath.Join(goBase, rel))
		files, size = files+1, size+len(a)
		if errA == nil && errB == nil && string(a) != string(b) {
			bad++
			t.Errorf("%s differs:\n python %s\n     go %s", rel, around(string(a), string(b)), around(string(b), string(a)))
		}
		return nil
	})
	return files, size
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
		writeFile(t, filepath.Join(base, filepath.FromSlash(rel)), []byte(text))
	}
	for _, d := range []string{"guides/Empty", "guides/Upper/toc.xml.d"} {
		if err := os.MkdirAll(filepath.Join(base, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
}

func TestParityGuidesSynthetic(t *testing.T) {
	driver(t)
	pyBase, goBase := filepath.Join(t.TempDir(), "py"), filepath.Join(t.TempDir(), "go")
	for _, b := range []string{pyBase, goBase} {
		guidesFixture(t, b)
	}
	for _, builder := range []string{"guides", "devguide", "guidesindex", "devguidetoc"} {
		runBoth(t, builder, pyBase, goBase)
	}
	// A second run replaces the clean trees and re-reads the index it wrote.
	for _, builder := range []string{"guides", "guidesindex"} {
		runBoth(t, builder, pyBase, goBase)
	}
}

// TestParityNonUTF8Names: page and folder names that are not UTF-8 make
// both parsers raise (the old clean tree is kept); guides-index.json spells
// them as Python's surrogate escapes, sorted as decoded names.
func TestParityNonUTF8Names(t *testing.T) {
	driver(t)
	pyBase, goBase := filepath.Join(t.TempDir(), "py"), filepath.Join(t.TempDir(), "go")
	for _, b := range []string{pyBase, goBase} {
		for _, rel := range []string{"guides/Odd\xff/a\xfe.html", "guides/Odd\xff/b.html", "guides/Odd\xff/\xee\x80\x80.html",
			"guides/Plain/c.html", "guides/\xee\x80\x80/d.html", "devguide/x\xff.html", "devguide-clean/old.txt",
			"guides-clean/old.txt"} {
			writeFile(t, filepath.Join(b, filepath.FromSlash(rel)), []byte("<title>T</title><p>x</p>"))
		}
	}
	for _, builder := range []string{"guides", "devguide", "guidesindex"} {
		runBoth(t, builder, pyBase, goBase)
	}
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
		writeFile(t, filepath.Join(install, "modules", "Upper.jar"), []byte("x"))
		writeFile(t, filepath.Join(install, "modules", "docToc-doc.jar"), []byte("x"))
	}
	reg := map[string]any{"versions": map[string]any{"9.9.9": map[string]any{
		"label": "test", "install_dir": install, "data_root": "versions/9.9.9"}}}
	data, err := json.Marshal(reg)
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(repo, "versions", "registry.json"), data)
	if manifest {
		writeFile(t, filepath.Join(repo, "versions", "9.9.9.manifest.json"),
			[]byte(`{"jars": [{"jar": "modules/docToc-doc.jar"}, {"jar": "modules/docSpy-doc.jar"}, {"jar": "x/docNone-doc.jar"}]}`))
	}
	// An earlier index whose values are kept when they cannot be checked.
	writeFile(t, filepath.Join(base, "indexes", "guides-index.json"),
		[]byte(`{"Spy": {"source_jar": "old-spy.jar"}, "Bad": {"source_jar": 0}, "Upper": "not a dict", "Empty": {}}`))
	return base
}

func TestParityGuidesIndexSources(t *testing.T) {
	driver(t)
	for _, c := range []struct {
		name              string
		mounted, manifest bool
	}{{"mounted", true, true}, {"unmounted", false, true}, {"no manifest", true, false}} {
		t.Run(c.name, func(t *testing.T) {
			pyBase := registeredFixture(t, filepath.Join(t.TempDir(), "py"), c.mounted, c.manifest)
			goBase := registeredFixture(t, filepath.Join(t.TempDir(), "go"), c.mounted, c.manifest)
			runBoth(t, "guidesindex", pyBase, goBase)
		})
	}
}

// TestParityTocFiles compares parse_toc and count_items on every toc.xml
// of the synthetic corpus plus the toc corners.
func TestParityTocFiles(t *testing.T) {
	driver(t)
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
	for name, text := range corners {
		p := filepath.Join(dir, name)
		writeFile(t, p, []byte(text))
		paths = append(paths, p)
	}
	py := runHTMLDriver(t, "", append([]string{"toc"}, paths...)...)
	var gol []string
	for _, p := range paths {
		toc, warning := ParseToc(p)
		rows := array()
		for _, k := range toc.Order {
			e := toc.Pages[k]
			trail := array()
			for _, s := range e.Trail {
				trail.A = append(trail.A, str(s))
			}
			rows.A = append(rows.A, array(str(k), str(e.Title), trail))
		}
		warnings := array()
		if warning != "" {
			warnings.A = append(warnings.A, str(warning))
		}
		gol = append(gol, dumpLine(str("toc"), rows, intV(CountItems(p)), warnings))
	}
	diffLines(t, "toc", py, gol)
}

// realBase links a real version's raw trees into a temporary registered
// data root, so both sides build next to the read-only corpus: the
// registry entry and the version manifest are copied, guides/ and
// devguide/ are symlinks.
func realBase(t *testing.T, vid string) string {
	t.Helper()
	help := niagaraHelpData()
	if help == "" {
		missingReference(t, "no niagara-help data checkout")
	}
	src := filepath.Join(help, "versions")
	reg, err := os.ReadFile(filepath.Join(src, "registry.json"))
	if err != nil {
		missingReference(t, fmt.Sprintf("no registry: %v", err))
	}
	if _, err := os.Stat(filepath.Join(src, vid, "guides")); err != nil {
		missingReference(t, fmt.Sprintf("version %s not extracted: %v", vid, err))
	}
	repo := t.TempDir()
	base := filepath.Join(repo, "versions", vid)
	writeFile(t, filepath.Join(repo, "versions", "registry.json"), reg)
	copyFile(t, filepath.Join(src, vid+".manifest.json"), filepath.Join(repo, "versions", vid+".manifest.json"))
	if err := os.MkdirAll(base, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, d := range []string{"guides", "devguide"} {
		if err := os.Symlink(filepath.Join(src, vid, d), filepath.Join(base, d)); err != nil {
			t.Fatal(err)
		}
	}
	return base
}

// TestParityGuidesReal builds guides-clean/, devguide-clean/,
// guides-index.json and devguide-toc.json of EC-Net 4.3.58.18 on both
// sides with HELPBUILD_PARITY_REAL=1; HELPBUILD_PARITY_415=1 adds
// 4.15.6.20.
func TestParityGuidesReal(t *testing.T) {
	needRealParity(t, "HELPBUILD_PARITY_415")
	driver(t)
	vids, err := guidesRealVersions()
	if err != nil {
		t.Fatal(err)
	}
	for _, vid := range vids {
		t.Run(vid, func(t *testing.T) {
			pyBase, goBase := realBase(t, vid), realBase(t, vid)
			for _, builder := range []string{"guides", "devguide", "guidesindex", "devguidetoc"} {
				runBoth(t, builder, pyBase, goBase)
			}
		})
	}
}

// runHTMLDriver runs htmldriver.py with stdin and returns its lines.
func runHTMLDriver(t *testing.T, stdin string, args ...string) []string {
	t.Helper()
	python, tools := driver(t)
	script, err := filepath.Abs(filepath.Join("testdata", "parity", "htmldriver.py"))
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(python, append([]string{"-I", "-B", script, tools}, args...)...)
	cmd.Stdin = strings.NewReader(stdin)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("htmldriver %v: %v\n%s", args, err, stderr.String())
	}
	return strings.Split(strings.TrimSuffix(string(out), "\n"), "\n")
}
