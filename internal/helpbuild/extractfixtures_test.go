package helpbuild

import (
	"archive/zip"
	"bytes"
	"path/filepath"
	"testing"

	"github.com/angeles725/niagara-go/internal/helpbuild/internal/testkit"
)

// Fixtures of the S2 parity tests (parity_extract_test.go): the synthetic
// install, the registry, and the full-tree listing both sides are compared
// by.

// writeStoredJar writes a jar whose entries are stored, not deflated.
func writeStoredJar(t *testing.T, path string, files ...[2]string) {
	t.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for _, f := range files {
		fw, err := w.CreateHeader(&zip.FileHeader{Name: f[0], Method: zip.Store})
		if err != nil {
			t.Fatal(err)
		}
		_, _ = fw.Write([]byte(f[1]))
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	testkit.WriteFile(t, path, buf.Bytes())
}

// syntheticInstall is a fake install covering every mapping of doc_jars:
// the test_doc_jar_completeness / test_extraction fixtures plus duplicate
// names (first wins), directories, odd UTF-8 names, CRLF text for the
// EOL-only class, a same-jar and a cross-jar collision, a source archive,
// stored entries, an unreadable module jar and the javadoc jar.
func syntheticInstall(t *testing.T, dir string) string {
	t.Helper()
	page := "<html><head><title>T</title></head><body><p>x</p></body></html>"
	install := filepath.Join(dir, "install")
	mods := filepath.Join(install, "modules")
	jars := map[string][][2]string{
		"docClHVACTest-doc.jar": {{"doc/toc.xml", "<toc/>"}, {"doc/index.html", page}, {"doc/sub/", ""},
			{"doc/Tasks.html", page}, {"doc/sub/Overview.html", page}, {"doc/README.txt", "readme quokka\n"},
			{"doc/Tasks.html", "duplicate, never extracted"}, {"doc/empty.txt", ""}, {"doc/crlf.txt", "a\r\nb\r\n"},
			{"doc/cl/hvac/BAbsolute.bajadoc", "<x/>"}, {"doc/Ünïcode ñ.html", page}, {"doc/日本語.md", "md"},
			{"doc/emoji\U0001F600.txt", "e"}, {"doc/a b/c.md", "c"}, {"META-INF/MANIFEST.MF", "Manifest-Version: 1.0\n"}},
		"docDeveloper-doc.jar": {{"doc/index.html", page}, {"doc/o.html", page}, {"doc/changes37.txt", "zebra\n"},
			{"doc/jsdoc/m/Apache-License-2.0.txt", "licence"}, {"rc/com/x/A.java", "class A {}\r\n"},
			{"rc/com/x/A.class", "\xca\xfe"}, {"other/x.txt", "x"}},
		"docDeveloper-ux.jar": {{"examples/bajaux/ZorbWidget.js", "define();"}, {"examples/bajaux/", ""},
			{"rc/Playground.built.min.js", "var a;"}, {"com/tridium/docdeveloper/ux/Ex.class", "\xca\xfe"},
			{"docDeveloper-ux.lexicon", "k=v\n"}, {"META-INF/MANIFEST.MF", "M\n"}},
		"docDeveloper-rt.jar": {{"rc/com/x/BVal.java", "class BVal {}\n"}, {"rc/com/x/BVal.class", "\xca\xfe"},
			{"docDeveloper-rt.lexicon", "k=v\n"}, {"module.palette", "<palette/>"}},
		"docSource-doc.jar": {{"baja-rt/javax/baja/sys/BSrc.java", "package javax.baja.sys;\n"},
			{"doc/index.html", page}, {"baja-rt/", ""}},
		"docJdk-doc.jar": {{"doc/jdk-se/java/util/HashMap.bajadoc", "<x/>"}, {"other.txt", "not mapped"}},
		"docFoo-doc.jar": {{"doc/x.html", "from docFoo-doc"}, {"y.txt", "kept relative"}},
		"docFoo.jar":     {{"doc/x.html", "from docFoo, overwrites"}},
		"ascHelp.jar": {{"doc/About-Asc.html", page}, {"doc/style.css", "p{}"}, {"doc/lgpl-2.1.txt", "GNU\n"},
			{"doc/libfoo-1.0.tar.xz", "\xfd7zXZ"}, {"com/acme/asc/BAsc.class", "\xca\xfe"},
			{"com/acme/asc/icons/asc.png", "\x89PNG"}, {"META-INF/MANIFEST.MF", "M\n"}},
		"thing-ux.jar": {{"rc/notes.md", "# Notes\n"}, {"doc/rc/notes.md", "# collides, skipped\n"},
			{"rc/node_modules/pkg/README.md", "npm\n"}, {"rc/thing.js", "var x;"}, {"jsdoc/a/b.html", page}},
		"plainModule-rt.jar": {{"com/acme/BPlain.class", "\xca\xfe"}, {"doc/style.css", "p {}\n"}},
		"workbench-wb.jar":   {{"com/tridium/workbench/media/about.html", page}, {"res/x/home.html", page}},
	}
	for name, files := range jars {
		writeJar(t, filepath.Join(mods, name), files...)
	}
	writeStoredJar(t, filepath.Join(mods, "docStored-doc.jar"), [2]string{"doc/s.txt", "stored\r\nline\r\n"},
		[2]string{"doc/z.html", page})
	testkit.WriteFile(t, filepath.Join(mods, "broken-rt.jar"), []byte("not a zip"))
	testkit.WriteFile(t, filepath.Join(mods, "notes.txt"), []byte("not a jar"))
	writeJar(t, filepath.Join(install, "javadoc", JavadocJar), [2]string{"doc/index.html", page},
		[2]string{"doc/javax/baja/sys/BObject.html", page}, [2]string{"other.txt", "x"})
	return install
}
