package helpbuild

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/angeles725/niagara-go/internal/help/fts"
	ftbuild "github.com/angeles725/niagara-go/internal/helpbuild/fulltext"
	"github.com/angeles725/niagara-go/internal/helpbuild/internal/testkit"
)

const linePx = "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<px version=\"1.0\">\n" +
	"  <content><ScrollPane><CanvasPane name=\"LineView\"/></ScrollPane></content>\n</px>\n"

// exampleInstall is lexiconInstall's registered version with module jars
// holding templates: a Px view and a Handlebars template (written), an XML
// bog (written), a zipped bog and a NUL-carrying Velocity file (no text), a
// vendored template (skipped), a script (not a template), a doc jar's Px
// page (extracted as a guide, not read here) and a jar that is not a zip.
func exampleInstall(t *testing.T) (root, install string) {
	t.Helper()
	root, install = lexiconInstall(t)
	mods := filepath.Join(install, "modules")
	testkit.WriteJar(t, filepath.Join(mods, "kitControl-rt.jar"),
		[2]string{"rc/examples/px/", ""}, [2]string{"rc/examples/px/LineView.px", linePx},
		[2]string{"rc/fe/templates/Row.hbs", "<tr>{{displayName}}</tr>\n"},
		[2]string{"rc/vendor/lib/x.hbs", "{{vendored}}"}, [2]string{"rc/app.js", "define([], 1);"})
	testkit.WriteJar(t, filepath.Join(mods, "tls-rt.jar"),
		[2]string{"profiles/Folder.bog", "PK\x03\x04zipped"},
		[2]string{"profiles/Console.bog", "\ufeff<?xml version=\"1.0\"?>\n<bajaObjectGraph version=\"4.0\"/>\n"},
		[2]string{"rc/page.vm", "<html>\x00</html>"})
	testkit.WriteJar(t, filepath.Join(mods, "docFoo-doc.jar"), [2]string{"doc/view.px", linePx})
	if err := os.WriteFile(filepath.Join(mods, "corrupt-rt.jar"), []byte("not a zip"), 0o644); err != nil {
		t.Fatal(err)
	}
	return root, install
}

// A page opens with a header naming the module jar and the entry, then the
// template as it is in the jar.
func TestModuleExamplePage(t *testing.T) {
	p, why := moduleExamplePage("kitControl-rt.jar", "rc/examples/px/LineView.px", []byte(linePx))
	want := "TITLE: kitControl-rt LineView.px\nMODULE: kitControl-rt\nENTRY: rc/examples/px/LineView.px\n\n" + linePx
	if why != "" || p.rel != "kitControl-rt/rc/examples/px/LineView.px" || p.text != want {
		t.Errorf("page %q %q:\n%s\nwant\n%s", p.rel, why, p.text, want)
	}
	for entry, data := range map[string]string{
		"a.bog": "PK\x03\x04", "b.bog": "\x00\x01binary", "c.vm": "a\x00b", "d.hbs": "caf\xe9",
	} {
		if _, why := moduleExamplePage("x-rt.jar", entry, []byte(data)); why == "" {
			t.Errorf("%s %q gave a page", entry, data)
		}
	}
	if _, why := moduleExamplePage("x-rt.jar", "a.bog", []byte("PK\x03\x04")); why != "not XML (binary/zip bog)" {
		t.Errorf("zip bog: %q", why)
	}
}

func TestBuildModuleExamples(t *testing.T) {
	root, install := exampleInstall(t)
	res, err := buildModuleExamples(root)
	if err != nil {
		t.Fatal(err)
	}
	pages := copiedFiles(t, filepath.Join(root, moduleExamplesDir))
	if want := []string{"kitControl-rt/rc/examples/px/LineView.px", "kitControl-rt/rc/fe/templates/Row.hbs",
		"tls-rt/profiles/Console.bog"}; !reflect.DeepEqual(pages, want) {
		t.Errorf("pages %q, want %q", pages, want)
	}
	line := "module-examples/: 3 templates (1 .px, 1 .hbs, 0 .vm, 1 .bog) of 6 template entries: 2 not text, " +
		"0 unreadable, 1 vendored (from " + install + "/modules, 8 jars, 1 unreadable)"
	if res.Files != 3 || !reflect.DeepEqual(res.Log, []string{line}) {
		t.Errorf("result %+v\nwant log %q", res, line)
	}
	mods := filepath.Join(install, "modules")
	wantWarn := []string{
		"WARNING: module_examples_extractor: " + filepath.Join(mods, "corrupt-rt.jar") + ": ",
		"WARNING: module_examples_extractor: tls-rt.jar!profiles/Folder.bog: not XML (binary/zip bog)",
		"WARNING: module_examples_extractor: tls-rt.jar!rc/page.vm: binary, not text (NUL byte or invalid UTF-8)",
	}
	if len(res.Warnings) != len(wantWarn) {
		t.Fatalf("warnings %q, want %q", res.Warnings, wantWarn)
	}
	for i, w := range wantWarn {
		if !strings.HasPrefix(res.Warnings[i], w) || i > 0 && res.Warnings[i] != w {
			t.Errorf("warning %d %q, want %q", i, res.Warnings[i], w)
		}
	}
	// The pages are a full-text source: find reaches them.
	if _, err := ftbuild.Build(root, ftbuild.Options{}); err != nil {
		t.Fatal(err)
	}
	docs, err := fulltextDocsAt(fts.DBPath(root))
	if err != nil || !docs["module-examples/kitControl-rt/rc/examples/px/LineView.px"] || !docs["module-examples/tls-rt/profiles/Console.bog"] {
		t.Errorf("full-text docs %v (%v)", docs, err)
	}
	// An install that is not mounted keeps the last copy.
	if err := os.RemoveAll(mods); err != nil {
		t.Fatal(err)
	}
	if res, err := buildModuleExamples(root); err != nil || len(copiedFiles(t, filepath.Join(root, moduleExamplesDir))) != 3 ||
		!strings.Contains(res.Log[0], "skipped (the last copy is kept)") {
		t.Errorf("unmounted install: %+v %v", res, err)
	}
}

// coverage --sweep classifies every template of a module jar: indexed when
// its module-examples/ page is in the full-text index, excluded when it has
// no text (a zipped bog) or is vendored, UNCLASSIFIED when it has text but no
// indexed page; kitControl's example Px views are no longer excluded by rule.
func TestSweepClassifiesModuleExamples(t *testing.T) {
	root, install := exampleInstall(t)
	if !mustDefault(t).IsDocLike("rc/examples/px/LineView.px") || mustDefault(t).IsDocLike("rc/fe/templates/Row.hbs") {
		t.Fatal("the fixture no longer covers a doc-like and a plain template")
	}
	testkit.WriteJar(t, filepath.Join(install, "sw", "1.0", "kitControl-rt.jar"), [2]string{"rc/a.px", linePx})
	if _, err := buildModuleExamples(root); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(root, moduleExamplesDir, "kitControl-rt", "rc", "fe", "templates", "Row.hbs")); err != nil {
		t.Fatal(err)
	}
	if _, err := ftbuild.Build(root, ftbuild.Options{}); err != nil {
		t.Fatal(err)
	}
	docs := func(string) (map[string]bool, error) { return fulltextDocsAt(fts.DBPath(root)) }
	res := scanOK(t, install, root, SweepOptions{Fulltext: docs})
	got := map[string]int{}
	for k, n := range res.Reasons {
		got[k.Rule] += n
	}
	if want := map[string]int{"example-not-text": 2, "example-vendored": 1, "shipped-file-outside-modules": 1}; got["example-not-text"] != 2 ||
		got["example-vendored"] != 1 || got["shipped-file-outside-modules"] != 1 || got["kitControl-examples"] != 0 {
		t.Errorf("exclusions %v, want %v", got, want)
	}
	var gaps []string
	for _, g := range res.Unclassified {
		if strings.HasPrefix(g.Jar, "modules/kitControl") || strings.HasPrefix(g.Jar, "modules/tls") {
			gaps = append(gaps, g.Jar+" "+g.Entry+" -- "+g.Why)
		}
	}
	want := []string{"modules/kitControl-rt.jar rc/fe/templates/Row.hbs -- module example is not indexed " +
		"(module-examples/kitControl-rt/rc/fe/templates/Row.hbs): run rebuild-all, then find"}
	if !reflect.DeepEqual(gaps, want) {
		t.Errorf("unclassified %q\nwant %q", gaps, want)
	}
	if row := res.Rows["modules/"]; row == nil || row.Indexed < 2 {
		t.Errorf("modules/ row %+v: LineView.px and Console.bog are indexed", row)
	}
}
