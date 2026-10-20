package helpbuild

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/angeles725/niagara-go/internal/help/fts"
	ftbuild "github.com/angeles725/niagara-go/internal/helpbuild/fulltext"
	"github.com/angeles725/niagara-go/internal/helpbuild/internal/testkit"
)

// Classification of corpus paths and unmapped entries (doc_types.classify,
// classify_unmapped, content_type), one row per branch.
func TestClassify(t *testing.T) {
	for _, c := range []struct{ rel, verdict, detail, ctype string }{
		{"devguide-examples/web/App.JS", verdictIndex, "example", "devguide-examples .js"},
		{"devguide-examples/ext/babel.js", verdictExclude, reasons["third-party-bundle"], "devguide-examples .js"},
		{"devguide-examples/a/b.MIN.JS", verdictExclude, reasons["minified"], "devguide-examples .js"},
		{"devguide-examples/s.css", verdictExclude, reasons["style"], "devguide-examples .css"},
		{"devguide-examples/x.lexicon", verdictExclude, guideJunk[".lexicon"], "devguide-examples .lexicon"},
		{"devguide-examples/data.bin", verdictUnknown, "", "devguide-examples .bin"},
		{"guides/A/IMG.PNG", verdictExclude, reasons["image"], "guides .png"},
		{"guides/A/f.WOFF2", verdictExclude, reasons["style"], "guides .woff2"},
		{"guides/A/app.js", verdictExclude, reasons["script"], "guides .js"},
		{"guides/A/s.dat", verdictExclude, reasons["search-index"], "guides .dat"},
		{"guides/A/LICENSE.txt", verdictExclude, reasons["licence"], "guides .txt"},
		{"guides/A/notes.txt", verdictIndex, "guide-page", "guides .txt"},
		{"guides/A/X.class", verdictExclude, reasons["class-file"], "guides .class"},
		{"bajadoc/index.html", verdictExclude, reasons["javadoc-nav"], "bajadoc .html"},
		{"bajadoc/p/class-use/B.html", verdictExclude, reasons["javadoc-nav"], "bajadoc .html"},
		{"bajadoc/p/B.html", verdictIndex, "javadoc-class", "bajadoc .html"},
		{"bajadoc/p/B.bajadoc", verdictUnknown, "", "bajadoc .bajadoc"},
		{"guides/A/module-index.bajadoc", verdictExclude, reasons["bajadoc-listing"], "guides .bajadoc"},
		{"jdk/java/lang/Object.bajadoc", verdictIndex, "jdk-xml", "jdk .bajadoc"},
		{"devguide/c.bajadoc", verdictIndex, "bajadoc-xml", "devguide .bajadoc"},
		{"source/c.bajadoc", verdictUnknown, "", "source .bajadoc"},
		{"devguide-rc/X.java", verdictIndex, "java", "devguide-rc .java"},
		{"source/doc/index.html", verdictExclude, reasons["source-nav"], "source .html"},
		{"source/doc/TOC.XML", verdictExclude, reasons["source-nav"], "source .xml"},
		{"source/x.txt", verdictUnknown, "", "source .txt"},
		{"devguide/toc.xml", verdictIndex, "devguide-toc", "devguide .xml"},
		{"devguide/sub/toc.xml", verdictUnknown, "", "devguide .xml"},
		{"guides/A/toc.xml", verdictIndex, "guide-toc", "guides .xml"},
		{"guides/A/sub/toc.xml", verdictUnknown, "", "guides .xml"},
		{"guides/A/src.TAR.XZ", verdictExclude, reasons["source-archive"], "guides .xz"},
		{"guides/A/Thumbs.db", verdictExclude, guideJunk[".db"], "guides .db"},
		{"guides/A/weird", verdictUnknown, "", "guides weird"},
		{"guides/A/.hidden", verdictUnknown, "", "guides .hidden"},
		{"other/x.html", verdictUnknown, "", "other .html"},
		{"bajadoc/p/Package-Summary.HTML", verdictExclude, reasons["javadoc-nav"], "bajadoc .html"},
		{"devguide/sub/x.txt", verdictIndex, "devguide-page", "devguide .txt"},
		{"devguide-examples/a/b.png", verdictUnknown, "", "devguide-examples .png"},
	} {
		v, d := classify(c.rel)
		if v != c.verdict || d != c.detail || contentType(c.rel) != c.ctype {
			t.Errorf("%s: %s %q %q; want %s %q %q", c.rel, v, d, contentType(c.rel), c.verdict, c.detail, c.ctype)
		}
	}
	for _, c := range []struct{ entry, verdict, detail string }{
		{"META-INF/MANIFEST.MF", verdictExclude, reasons["meta-inf"]},
		{"web/Foo.class", verdictExclude, reasons["class-file"]},
		{"lex/x.LEXICON", verdictExclude, guideJunk[".lexicon"]},
		{"README", verdictUnknown, ""},
	} {
		if v, d := classifyUnmapped(c.entry); v != c.verdict || d != c.detail {
			t.Errorf("unmapped %s: %s %q; want %s %q", c.entry, v, d, c.verdict, c.detail)
		}
	}
}

// An example entry needs a full-text reader: none is an error, never a
// silent "not indexed".
func TestScanDocJarsNeedsFulltextForExamples(t *testing.T) {
	dir := t.TempDir()
	jar := filepath.Join(dir, "inst", "modules", "docDeveloper-ux.jar")
	testkit.WriteJar(t, jar, [2]string{"web/app.js", "a"})
	testkit.WriteFile(t, filepath.Join(dir, "base", "devguide-examples", "web", "app.js"), []byte("a"))
	if _, err := scanDocJars("", filepath.Join(dir, "base"), []string{jar}, nil); !errors.Is(err, errNoFulltext) {
		t.Fatalf("no reader: %v", err)
	}
	docs := func(string) (map[string]bool, error) {
		return map[string]bool{"devguide-examples/web/app.js": true}, nil
	}
	s, err := scanDocJars("", filepath.Join(dir, "base"), []string{jar}, docs)
	if err != nil || len(s.Gaps) != 0 || s.Rows[docJarKey{"docDeveloper-ux.jar", "devguide-examples .js"}].Indexed != 1 {
		t.Fatalf("with a reader: %+v %v", s, err)
	}
}

// docJarGoldenCases are the gate goldens on the synthetic install.
func docJarGoldenCases(t *testing.T) (cases []covGoldenCase, pairs []string) {
	t.Helper()
	dir := t.TempDir()
	install, base := covInstall(t, dir), covBase(t, dir)
	repo := covRepo(t, dir, "4.99.1.2", install, base)
	return []covGoldenCase{
		{"docjars", "docjars", []string{repo, base, "4.99.1.2"}},
		{"docjars-no-install", "docjars", []string{repo, base, "9.9.9.9"}},
	}, []string{install, "<install>", base, "<base>", repo, "<repo>"}
}

// TestDocJarCoverageGolden: the Go gate on the synthetic install against
// its golden files.
func TestDocJarCoverageGolden(t *testing.T) {
	cases, pairs := docJarGoldenCases(t)
	checkCovGoldens(t, cases, pairs...)
}

const bajadocXML = `<?xml version="1.0"?><bajadoc><class qualifiedName="p.q.B" name="B" packageName="p.q"/></bajadoc>`

// Indexed.check: each kind is searchable only where its index says so.
func TestIndexedViewsCheck(t *testing.T) {
	root := t.TempDir()
	w := func(rel, data string) {
		testkit.WriteFile(t, filepath.Join(root, filepath.FromSlash(rel)), []byte(data))
	}
	w("indexes/class-index.json", `{"A": {"package": "p.q", "others": [{"package": "r"}]}, "B": {"package": "p.q"}}`)
	w("indexes/jdk-class-index.json", `{"Object": {"package": "java.lang"}}`)
	w("indexes/source-index.json", `{"x": {"path": "a/./B.java"}}`)
	w("indexes/guides-index.json", `{"G": {"toc": [1]}, "H": {"toc": []}}`)
	w("indexes/devguide-toc.json", `{"sections": [1]}`)
	w("devguide/javax/B.bajadoc", bajadocXML)
	w("devguide/bad.bajadoc", "<bajadoc><class")
	w("jdk/Object.bajadoc", `<bajadoc><class qualifiedName="java.lang.Object" name="Object" packageName="java.lang"/></bajadoc>`)
	w("devguide/page.html", "<p>x")
	w("devguide-clean/page.txt", "TITLE: x\n")
	w("guides/G/a.md", "# a")
	w("guides/G/b.html", "<p>b")
	w("guides-clean/G/a.md.txt", "TITLE: a\n")
	w("devguide-examples/x/demo.js", "function demo() {}\n")
	w("devguide-examples/x/other.js", "function other() {}\n")
	if _, err := ftbuild.Build(root, ftbuild.Options{}); err != nil {
		t.Fatal(err)
	}
	w("devguide-examples/x/late.js", "function late() {}\n") // after the build: the index is stale

	x := newIndexedViews(root, func(string) (map[string]bool, error) { return fulltextDocsAt(fts.DBPath(root)) })
	for _, c := range []struct {
		kind, rel string
		want      bool
	}{
		{"javadoc-class", "bajadoc/p/q/A.html", true},
		{"javadoc-class", "bajadoc/r/A.html", true},
		{"javadoc-class", "bajadoc/p/q/C.html", false},
		{"bajadoc-xml", "devguide/javax/B.bajadoc", true},
		{"bajadoc-xml", "devguide/bad.bajadoc", false},
		{"jdk-xml", "jdk/Object.bajadoc", true},
		{"bajadoc-xml", "jdk/Object.bajadoc", false},
		{"devguide-page", "devguide/page.html", true},
		{"guide-page", "guides/G/a.md", true},
		{"guide-page", "guides/G/b.html", false},
		{"guide-toc", "guides/G/toc.xml", true},
		{"guide-toc", "guides/H/toc.xml", false},
		{"devguide-toc", "devguide/toc.xml", true},
		{"java", "source/a/B.java", true},
		{"java", "source/a/C.java", false},
		{"example", "devguide-examples/x/demo.js", true},
		{"example", "devguide-examples/x/late.js", false},
		{"no-such-kind", "guides/G/a.md", false},
	} {
		got, err := x.check(c.kind, c.rel)
		if err != nil || got != c.want {
			t.Errorf("Check(%q, %q) = %v, %v; want %v", c.kind, c.rel, got, err, c.want)
		}
	}
}
