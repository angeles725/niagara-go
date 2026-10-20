package helpbuild

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/angeles725/niagara-go/internal/helpbuild/internal/testkit"
	"github.com/angeles725/niagara-go/internal/pyjson"
)

// Ported from niagara-help tests/test_n3_advisories.py
// TestGuideSourceJarKeptWhenUnverifiable (method names cited), plus the
// index shape and devguide-toc.

// keptFixture is TestGuideSourceJarKeptWhenUnverifiable.setUp.
func keptFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	testkit.WriteFile(t, filepath.Join(root, "guides", "ascCommon", "index.html"), []byte("<p>x"))
	testkit.WriteFile(t, filepath.Join(root, "indexes", "guides-index.json"), []byte(`{"ascCommon": {"source_jar": "ascCommon.jar"}}`))
	return root
}

func sourceJar(t *testing.T, index pyjson.Value, folder string) pyjson.Value {
	t.Helper()
	e, ok := index.Get(folder)
	if !ok {
		t.Fatalf("no %s entry", folder)
	}
	v, _ := e.Get("source_jar")
	return v
}

func TestGuidesIndexKeepsUnverifiableJar(t *testing.T) {
	root := keptFixture(t)
	// test_unmounted_install_keeps_the_recorded_jar
	index, kept, _, err := GuidesIndex(root, JarSources{Names: []string{}})
	if err != nil {
		t.Fatal(err)
	}
	if v := sourceJar(t, index, "ascCommon"); v.S != "ascCommon.jar" || !reflect.DeepEqual(kept, []string{"ascCommon"}) {
		t.Errorf("source_jar = %+v, kept %q", v, kept)
	}
	// test_mounted_install_without_the_jar_is_null
	modules := filepath.Join(root, "install", "modules")
	if err := os.MkdirAll(modules, 0o755); err != nil {
		t.Fatal(err)
	}
	index, kept, _, err = GuidesIndex(root, JarSources{Names: []string{}, ModulesDir: modules})
	if err != nil {
		t.Fatal(err)
	}
	if v := sourceJar(t, index, "ascCommon"); v.Kind != pyjson.Null || kept != nil {
		t.Errorf("source_jar = %+v, kept %q", v, kept)
	}
	// The module jar itself when modules/ holds it.
	testkit.WriteFile(t, filepath.Join(modules, "ascCommon.jar"), []byte("x"))
	index, _, _, err = GuidesIndex(root, JarSources{Names: []string{}, ModulesDir: modules})
	if err != nil {
		t.Fatal(err)
	}
	if v := sourceJar(t, index, "ascCommon"); v.S != "ascCommon.jar" {
		t.Errorf("source_jar = %+v", v)
	}
}

// test_main_reports_the_kept_values: an unregistered root has no manifest,
// so the earlier value is kept and reported.
func TestBuildGuidesIndexReportsKept(t *testing.T) {
	root := keptFixture(t)
	res, err := BuildGuidesIndex(root)
	if err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(root, "indexes", "guides-index.json")
	want := []string{"guides-index.json: 1 folders -> " + out,
		"  NOTE: 1 source_jar value(s) kept from the previous index, not re-verified (install not mounted or no version manifest): ascCommon"}
	if !reflect.DeepEqual(res.Log, want) {
		t.Errorf("Log = %q", res.Log)
	}
	wantJSON := "{\n  \"ascCommon\": {\n    \"html_count\": 1,\n    \"source_jar\": \"ascCommon.jar\",\n" +
		"    \"topics\": [\n      \"index\"\n    ],\n    \"total_topics\": 1\n  }\n}"
	if got := testkit.ReadText(t, out); got != wantJSON {
		t.Errorf("guides-index.json = %q", got)
	}
}

func TestGuidesIndexEntry(t *testing.T) {
	root := t.TempDir()
	g := filepath.Join(root, "guides", "Foo")
	testkit.WriteFile(t, filepath.Join(g, "TOC.xml"), []byte(`<toc><tocitem target="a.html" text="A page">`+
		`<tocitem target="sub/b.htm" text="B"/></tocitem><tocitem target="gone.html" text="G"/></toc>`))
	for _, rel := range []string{"a.html", "a.htm", "sub/b.htm", "notes.md", "LICENSE.txt", "style.css", "x.HTML"} {
		testkit.WriteFile(t, filepath.Join(g, filepath.FromSlash(rel)), []byte("<p>x"))
	}
	index, kept, warnings, err := GuidesIndex(root, JarSources{Names: []string{"docFoo-doc.jar"}})
	if err != nil || kept != nil || warnings != nil {
		t.Fatalf("err %v, kept %q, warnings %q", err, kept, warnings)
	}
	got := pyjson.Dumps(index, pyjson.DumpOpts{EnsureASCII: true, Indent: -1, ItemSep: ", ", KeySep: ": "})
	want := `{"Foo": {"html_count": 4, "source_jar": "docFoo-doc.jar", "topics": ["a", "a", "notes.md", "sub/b", "x"], ` +
		`"total_topics": 5, "toc": {"file": "TOC.xml", "items": 3, "titled": 2}, "titles": {"a": "A page", "sub/b": "B"}}}`
	if got != want {
		t.Errorf("index =\n %s\nwant\n %s", got, want)
	}
	if TopicName("sub/P.HTM") != "sub/P" || TopicName("r.txt") != "r.txt" {
		t.Error("TopicName")
	}
}

// devguideIndex is a devguide/index.html with every build_devguide_toc
// branch: a commented-out section, external links, a multi-line
// description, a heading whose only link joins the previous section, an
// empty heading and a nested list.
const devguideIndex = `<html><body>
<!-- <h1 id='hidden'>Quick Links</h1><li><a href='q.html'>Q</a></li> -->
<h1 id='intro'>Intro &amp; overview</h1>
<ul><li><a href='a.html'><font class='index'>Alpha</font></a>: the first page.</li>
<li><a href="http://example.com/x">External</a> skipped</li>
<li><a href='b.html'>Beta &#8212; two</a>
  description
  spanning lines.</li></ul>
<h1 id="rpc" >Niagara RPC</h1>
<p>For RPC <a href='https://x'>see</a> or <a href='rpc.html'>click here</a>.</p>
<h1 id='empty'>Nothing here</h1>
<h1 id='ui'>UI</h1>
<ul><li><a href='ui.html'>UI</a><ul><li><a href='ui/sub.html'>Sub</a> nested</li></ul></li></ul>
</body></html>`

func TestDevguideTOC(t *testing.T) {
	root := t.TempDir()
	testkit.WriteFile(t, filepath.Join(root, "devguide", "index.html"), []byte(devguideIndex))
	toc, err := DevguideTOC(root)
	if err != nil {
		t.Fatal(err)
	}
	got := pyjson.Dumps(toc, pyjson.DumpOpts{EnsureASCII: false, Indent: -1, ItemSep: ", ", KeySep: ": "})
	for _, want := range []string{
		`{"id": "intro", "title": "Intro & overview", "items": [{"file": "a.html", "title": "Alpha", "description": "the first page"}, ` +
			`{"file": "b.html", "title": "Beta — two", "description": "description spanning lines"}, ` +
			`{"file": "rpc.html", "title": "Niagara RPC", "description": "For RPC see or click here"}]}`,
		`{"id": "ui", "title": "UI", "items": [{"file": "ui.html", "title": "UI", "description": "Sub nested"}]}`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("toc lacks %s:\n%s", want, got)
		}
	}
	if strings.Contains(got, "hidden") || strings.Contains(got, "example.com") || strings.Contains(got, `"empty"`) {
		t.Errorf("comment, external link or empty section kept:\n%s", got)
	}
	if _, err := DevguideTOC(t.TempDir()); err == nil {
		t.Error("a missing devguide/index.html is an error (Python raises)")
	}
}

// known_jar_names on every manifest shape: the outcome (names, or an
// error where Python raises) was taken from CPython 3.15 running
// build_guides_index.known_jar_names on the same files.
func TestKnownJarNamesManifestShapes(t *testing.T) {
	for _, c := range []struct {
		manifest string
		want     string // names joined by " "; "raise" for an error
	}{
		{`[]`, "raise"}, // AttributeError: a list has no .get
		{`{}`, ""},
		{`{"jars": {}}`, ""},
		{`{"jars": ""}`, ""},
		{`{"jars": []}`, ""},
		{`{"jars": null}`, "raise"}, // TypeError: None is not iterable
		{`{"jars": 0}`, "raise"},
		{`{"jars": 5}`, "raise"},
		{`{"jars": "ab"}`, "raise"},       // TypeError: a str record
		{`{"jars": {"a": 1}}`, "raise"},   // TypeError: a str key as the record
		{`{"jars": [{"x": 1}]}`, "raise"}, // KeyError
		{`{"jars": [{"jar": 5}]}`, "raise"},
		{`{"jars": ["s"]}`, "raise"},
		{`{"jars": [{"jar": "m/b.jar"}, {"jar": "a.jar"}]}`, "a.jar b.jar"},
		{`not json`, "raise"},
	} {
		repo := t.TempDir()
		writeRegistry(t, repo, map[string][3]string{"9.9": {"L", "/nonexistent", "versions/9.9"}})
		base := filepath.Join(repo, "versions", "9.9")
		if err := os.MkdirAll(base, 0o755); err != nil {
			t.Fatal(err)
		}
		testkit.WriteFile(t, filepath.Join(repo, "versions", "9.9.manifest.json"), []byte(c.manifest))
		names, ok, err := KnownJarNames(base)
		got := strings.Join(names, " ")
		if err != nil {
			got = "raise"
		} else if !ok || names == nil {
			got = fmt.Sprintf("ok=%v names=%q", ok, names)
		}
		if got != c.want {
			t.Errorf("manifest %s: %s (err %v), want %s", c.manifest, got, err, c.want)
		}
	}
	// No manifest at all is Python's None.
	repo := t.TempDir()
	writeRegistry(t, repo, map[string][3]string{"9.9": {"L", "/nonexistent", "versions/9.9"}})
	if names, ok, err := KnownJarNames(filepath.Join(repo, "versions", "9.9")); ok || names != nil || err != nil {
		t.Errorf("no manifest: %q %v %v", names, ok, err)
	}
}
