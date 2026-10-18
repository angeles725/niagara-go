package helpbuild

import (
	"errors"
	"path/filepath"
	"testing"
)

// Classification of corpus paths and unmapped entries (doc_types.classify,
// classify_unmapped, content_type), one row per branch.
func TestClassify(t *testing.T) {
	for _, c := range []struct{ rel, verdict, detail, ctype string }{
		{"devguide-examples/web/App.JS", VerdictIndex, "example", "devguide-examples .js"},
		{"devguide-examples/ext/babel.js", VerdictExclude, Reasons["third-party-bundle"], "devguide-examples .js"},
		{"devguide-examples/a/b.MIN.JS", VerdictExclude, Reasons["minified"], "devguide-examples .js"},
		{"devguide-examples/s.css", VerdictExclude, Reasons["style"], "devguide-examples .css"},
		{"devguide-examples/x.lexicon", VerdictExclude, guideJunk[".lexicon"], "devguide-examples .lexicon"},
		{"devguide-examples/data.bin", VerdictUnknown, "", "devguide-examples .bin"},
		{"guides/A/IMG.PNG", VerdictExclude, Reasons["image"], "guides .png"},
		{"guides/A/f.WOFF2", VerdictExclude, Reasons["style"], "guides .woff2"},
		{"guides/A/app.js", VerdictExclude, Reasons["script"], "guides .js"},
		{"guides/A/s.dat", VerdictExclude, Reasons["search-index"], "guides .dat"},
		{"guides/A/LICENSE.txt", VerdictExclude, Reasons["licence"], "guides .txt"},
		{"guides/A/notes.txt", VerdictIndex, "guide-page", "guides .txt"},
		{"guides/A/X.class", VerdictExclude, Reasons["class-file"], "guides .class"},
		{"bajadoc/index.html", VerdictExclude, Reasons["javadoc-nav"], "bajadoc .html"},
		{"bajadoc/p/class-use/B.html", VerdictExclude, Reasons["javadoc-nav"], "bajadoc .html"},
		{"bajadoc/p/B.html", VerdictIndex, "javadoc-class", "bajadoc .html"},
		{"bajadoc/p/B.bajadoc", VerdictUnknown, "", "bajadoc .bajadoc"},
		{"guides/A/module-index.bajadoc", VerdictExclude, Reasons["bajadoc-listing"], "guides .bajadoc"},
		{"jdk/java/lang/Object.bajadoc", VerdictIndex, "jdk-xml", "jdk .bajadoc"},
		{"devguide/c.bajadoc", VerdictIndex, "bajadoc-xml", "devguide .bajadoc"},
		{"source/c.bajadoc", VerdictUnknown, "", "source .bajadoc"},
		{"devguide-rc/X.java", VerdictIndex, "java", "devguide-rc .java"},
		{"source/doc/index.html", VerdictExclude, Reasons["source-nav"], "source .html"},
		{"source/doc/TOC.XML", VerdictExclude, Reasons["source-nav"], "source .xml"},
		{"source/x.txt", VerdictUnknown, "", "source .txt"},
		{"devguide/toc.xml", VerdictIndex, "devguide-toc", "devguide .xml"},
		{"devguide/sub/toc.xml", VerdictUnknown, "", "devguide .xml"},
		{"guides/A/toc.xml", VerdictIndex, "guide-toc", "guides .xml"},
		{"guides/A/sub/toc.xml", VerdictUnknown, "", "guides .xml"},
		{"guides/A/src.TAR.XZ", VerdictExclude, Reasons["source-archive"], "guides .xz"},
		{"guides/A/Thumbs.db", VerdictExclude, guideJunk[".db"], "guides .db"},
		{"guides/A/weird", VerdictUnknown, "", "guides weird"},
		{"guides/A/.hidden", VerdictUnknown, "", "guides .hidden"},
		{"other/x.html", VerdictUnknown, "", "other .html"},
	} {
		v, d := Classify(c.rel)
		if v != c.verdict || d != c.detail || ContentType(c.rel) != c.ctype {
			t.Errorf("%s: %s %q %q; want %s %q %q", c.rel, v, d, ContentType(c.rel), c.verdict, c.detail, c.ctype)
		}
	}
	for _, c := range []struct{ entry, verdict, detail string }{
		{"META-INF/MANIFEST.MF", VerdictExclude, Reasons["meta-inf"]},
		{"web/Foo.class", VerdictExclude, Reasons["class-file"]},
		{"lex/x.LEXICON", VerdictExclude, guideJunk[".lexicon"]},
		{"README", VerdictUnknown, ""},
	} {
		if v, d := ClassifyUnmapped(c.entry); v != c.verdict || d != c.detail {
			t.Errorf("unmapped %s: %s %q; want %s %q", c.entry, v, d, c.verdict, c.detail)
		}
	}
}

func TestPyNormpath(t *testing.T) {
	for in, want := range map[string]string{
		"": ".", "source/../devguide-rc/X.java": "devguide-rc/X.java", "source/./a//b": "source/a/b",
		"//a/b": "//a/b", "///a": "/a", "../../x": "../../x", "/..": "/",
		// posixpath on every OS: a backslash is a name character, a drive
		// letter is a name, two leading slashes are kept
		`a\b/../c`: "c", `\\srv\share/x/..`: `\\srv\share`, "C:/x/../y": "C:/y", "//a/../b": "//b",
	} {
		if got := pyNormpath(in); got != want {
			t.Errorf("pyNormpath(%q) = %q, want %q", in, got, want)
		}
	}
}

// An example entry needs a full-text reader: none is an error, never a
// silent "not indexed".
func TestScanDocJarsNeedsFulltextForExamples(t *testing.T) {
	dir := t.TempDir()
	jar := filepath.Join(dir, "inst", "modules", "docDeveloper-ux.jar")
	writeJar(t, jar, [2]string{"web/app.js", "a"})
	writeFile(t, filepath.Join(dir, "base", "devguide-examples", "web", "app.js"), []byte("a"))
	if _, err := ScanDocJars("", filepath.Join(dir, "base"), []string{jar}, nil); !errors.Is(err, errNoFulltext) {
		t.Fatalf("no reader: %v", err)
	}
	docs := func(string) (map[string]bool, error) {
		return map[string]bool{"devguide-examples/web/app.js": true}, nil
	}
	s, err := ScanDocJars("", filepath.Join(dir, "base"), []string{jar}, docs)
	if err != nil || len(s.Gaps) != 0 || s.Rows[DocJarKey{"docDeveloper-ux.jar", "devguide-examples .js"}].Indexed != 1 {
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

// TestDocJarCoverageGolden: no Python needed; the Go gate on the synthetic
// install against its golden files.
func TestDocJarCoverageGolden(t *testing.T) {
	cases, pairs := docJarGoldenCases(t)
	checkCovGoldens(t, cases, false, pairs...)
}

// TestParityDocJarCoverageGolden: the golden files are Python's output.
func TestParityDocJarCoverageGolden(t *testing.T) {
	covPython(t)
	cases, pairs := docJarGoldenCases(t)
	checkCovGoldens(t, cases, true, pairs...)
}
