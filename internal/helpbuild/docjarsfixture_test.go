package helpbuild

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/angeles725/niagara-go/internal/helpbuild/internal/testkit"
)

// The synthetic install and data root of the doc-jar gate tests: one entry
// per branch of the gate (every index kind indexed, not indexed and not
// extracted, every exclusion reason, unmapped and unknown entries, an
// unreadable jar, a module help jar), names that need code-point padding
// and cutting.

// covBajadoc is one .bajadoc document holding the classes "pkg.Name".
func covBajadoc(classes ...string) string {
	var b strings.Builder
	b.WriteString("<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<bajadoc version=\"2.0\">\n")
	for _, q := range classes {
		i := strings.LastIndex(q, ".")
		fmt.Fprintf(&b, "<class module=\"m\" runtimeProfile=\"rt\" qualifiedName=\"%s\" name=\"%s\" packageName=\"%s\">"+
			"<description>d</description></class>\n", q, q[i+1:], q[:i])
	}
	b.WriteString("</bajadoc>\n")
	return b.String()
}

// longJar is a doc jar whose name is longer than the 34-column jar field,
// with non-ASCII letters around the cut.
const longJar = "docÜberlangerHandbuchÆØÅnameMitUmlautenÖ-doc.jar"

// covInstall writes the synthetic install under dir and returns its path.
func covInstall(t *testing.T, dir string) string {
	t.Helper()
	inst := filepath.Join(dir, "install")
	mod := filepath.Join(inst, "modules")
	writeJar(t, filepath.Join(mod, "docAlpha-doc.jar"),
		[2]string{"META-INF/MANIFEST.MF", "m"},
		[2]string{"META-INF/x.averyveryverylongextension", "x"},
		[2]string{"doc/", ""},
		[2]string{"doc/page.html", "<html>p</html>"},
		[2]string{"doc/page.html", "<html>duplicate</html>"},
		[2]string{"doc/missing.html", "m"},
		[2]string{"doc/noclean.htm", "n"},
		[2]string{"doc/notes.html", "n"},
		[2]string{"doc/notes.txt", "n"},
		[2]string{"doc/readme.md", "r"},
		[2]string{"doc/LICENSE.txt", "l"},
		[2]string{"doc/img/a.png", "p"},
		[2]string{"doc/IMG.PNG", "p"},
		[2]string{"doc/style.css", "c"},
		[2]string{"doc/font.WOFF2", "f"},
		[2]string{"doc/app.js", "j"},
		[2]string{"doc/search.dat", "d"},
		[2]string{"doc/toc.xml", "<toc/>"},
		[2]string{"doc/sub/toc.xml", "<toc/>"},
		[2]string{"doc/Thumbs.db", "t"},
		[2]string{"doc/x.bajadoc", covBajadoc("cl.alpha.BX")},
		[2]string{"doc/y.bajadoc", covBajadoc("cl.alpha.BY", "cl.alpha.BX")},
		[2]string{"doc/broken.bajadoc", "<bajadoc><class"},
		[2]string{"doc/empty.bajadoc", covBajadoc()},
		[2]string{"doc/module-index.bajadoc", "<bajadoc/>"},
		[2]string{"doc/src.tar.gz", "s"},
		[2]string{"doc/weird", "w"},
		[2]string{"doc/.hidden", "h"},
		[2]string{"doc/über ñame.html", "u"},
	)
	writeJar(t, filepath.Join(mod, "docBeta-doc.jar"), [2]string{"doc/b.html", "b"})
	writeJar(t, filepath.Join(mod, longJar), [2]string{"doc/l.html", "l"}, [2]string{"doc/l.gif", "g"})
	writeJar(t, filepath.Join(mod, "docDeveloper-doc.jar"),
		[2]string{"doc/toc.xml", "<toc/>"},
		[2]string{"doc/a.html", "a"},
		[2]string{"doc/b.txt", "b"},
		[2]string{"doc/sub/toc.xml", "<toc/>"},
		[2]string{"doc/sub/c.html", "c"},
		[2]string{"doc/c.bajadoc", covBajadoc("javax.baja.dev.BDev")},
		[2]string{"doc/package-index.bajadoc", "<bajadoc/>"},
		[2]string{"rc/X.java", "class X {}"},
		[2]string{"rc/Missing.java", "class M {}"},
		[2]string{"other.txt", "o"},
	)
	writeJar(t, filepath.Join(mod, "docDeveloper-ux.jar"),
		[2]string{"web/app.js", "var a = 1;"},
		[2]string{"web/App.JS", "var b = 2;"},
		[2]string{"web/view.hbs", "{{x}}"},
		[2]string{"web/gone.px", "<px/>"},
		[2]string{"ext/babel.js", "babel"},
		[2]string{"web/app.min.js", "m"},
		[2]string{"web/s.css", "c"},
		[2]string{"web/x.lexicon", "l"},
		[2]string{"web/data.bin", "d"},
		[2]string{"web/Foo.class", "\xca\xfe"},
	)
	writeJar(t, filepath.Join(mod, "docDeveloper-rt.jar"),
		[2]string{"rc/Y.java", "class Y {}"},
		[2]string{"rc/Y.class", "\xca\xfe"},
		[2]string{"lex/x.lexicon", "l"},
		[2]string{"a/b.txt", "b"},
		[2]string{"README", "r"},
	)
	writeJar(t, filepath.Join(mod, "docSource-doc.jar"),
		[2]string{"javax/A.java", "class A {}"},
		[2]string{"javax/B.java", "class B {}"},
		[2]string{"doc/index.html", "i"},
		[2]string{"doc/toc.xml", "<toc/>"},
		[2]string{"x.txt", "x"},
	)
	writeJar(t, filepath.Join(mod, "docJdk-doc.jar"),
		[2]string{"doc/java/lang/Object.bajadoc", covBajadoc("java.lang.Object")},
		[2]string{"doc/java/lang/String.bajadoc", covBajadoc("java.lang.String")},
		[2]string{"notdoc.txt", "n"},
	)
	testkit.WriteFile(t, filepath.Join(mod, "docBroken-doc.jar"), []byte("not a zip"))
	writeJar(t, filepath.Join(mod, "alarm-rt.jar"),
		[2]string{"META-INF/MANIFEST.MF", "m"},
		[2]string{"com/acme/BAlarm.class", "\xca\xfe"},
		[2]string{"doc/alarm.html", "a"},
		[2]string{"doc/alarm.css", "c"},
	)
	writeJar(t, filepath.Join(mod, "plain-rt.jar"), [2]string{"com/acme/BPlain.class", "\xca\xfe"})
	writeJar(t, filepath.Join(inst, "javadoc", JavadocJar),
		[2]string{"doc/index.html", "i"},
		[2]string{"doc/package-list", "javax.baja.sys"},
		[2]string{"doc/top.html", "t"},
		[2]string{"doc/javax/baja/sys/BObject.html", "o"},
		[2]string{"doc/javax/baja/sys/BOther.html", "o"},
		[2]string{"doc/javax/baja/sys/BMissing.html", "m"},
		[2]string{"doc/javax/baja/sys/BAbsent.html", "a"},
		[2]string{"doc/javax/baja/sys/class-use/BObject.html", "u"},
		[2]string{"doc/index-files/index-1.html", "i"},
		[2]string{"doc/javax/baja/sys/notes.xml", "n"},
		[2]string{"doc/javax/baja/sys/package-summary.html", "s"},
	)
	return inst
}

// covExtracted are the corpus files of the data root (the entries a jar
// maps there, minus those left out to be "not extracted").
var covExtracted = []string{
	"guides/Alpha/page.html", "guides/Alpha/noclean.htm", "guides/Alpha/notes.html", "guides/Alpha/notes.txt",
	"guides/Alpha/readme.md", "guides/Alpha/LICENSE.txt", "guides/Alpha/img/a.png", "guides/Alpha/IMG.PNG",
	"guides/Alpha/style.css", "guides/Alpha/font.WOFF2", "guides/Alpha/app.js", "guides/Alpha/search.dat",
	"guides/Alpha/toc.xml", "guides/Alpha/sub/toc.xml", "guides/Alpha/Thumbs.db", "guides/Alpha/weird",
	"guides/Alpha/.hidden", "guides/Alpha/über ñame.html", "guides/Alpha/module-index.bajadoc",
	"guides/Beta/b.html", "guides/ÜberlangerHandbuchÆØÅnameMitUmlautenÖ/l.html",
	"devguide/toc.xml", "devguide/a.html", "devguide/b.txt", "devguide/sub/toc.xml", "devguide/sub/c.html",
	"devguide-rc/X.java", "devguide-examples/web/app.js", "devguide-examples/web/App.JS",
	"devguide-examples/web/view.hbs", "devguide-examples/ext/babel.js", "devguide-examples/web/data.bin",
	"devguide-rc/Y.java", "source/javax/A.java", "source/doc/index.html", "source/x.txt",
	"jdk/java/lang/Object.bajadoc", "guides/alarm-rt/alarm.html",
	"bajadoc/index.html", "bajadoc/javax/baja/sys/BObject.html", "bajadoc/javax/baja/sys/BOther.html",
	"bajadoc/javax/baja/sys/BAbsent.html", "bajadoc/javax/baja/sys/notes.xml",
}

// covBase writes the synthetic data root under dir and returns its path.
func covBase(t *testing.T, dir string) string {
	t.Helper()
	base := filepath.Join(dir, "base")
	for _, rel := range covExtracted {
		testkit.WriteFile(t, filepath.Join(base, rel), []byte("text "+rel+"\n"))
	}
	files := map[string]string{
		"guides/Alpha/x.bajadoc":       covBajadoc("cl.alpha.BX"),
		"guides/Alpha/y.bajadoc":       covBajadoc("cl.alpha.BY", "cl.alpha.BX"),
		"guides/Alpha/broken.bajadoc":  "<bajadoc><class",
		"guides/Alpha/empty.bajadoc":   covBajadoc(),
		"devguide/c.bajadoc":           covBajadoc("javax.baja.dev.BDev"),
		"jdk/java/lang/String.bajadoc": covBajadoc("java.lang.String"),
		// guides-clean: page.txt, notes.txt (the .html's), notes.txt.txt,
		// readme.md.txt, über ñame.txt; not noclean.htm.txt
		"guides-clean/Alpha/page.txt":      "TITLE: p\n",
		"guides-clean/Alpha/notes.txt":     "TITLE: n\n",
		"guides-clean/Alpha/notes.txt.txt": "TITLE: n\n",
		"guides-clean/Alpha/readme.md.txt": "TITLE: r\n",
		"guides-clean/Alpha/über ñame.txt": "TITLE: u\n",
		"guides-clean/Beta/b.txt":          "TITLE: b\n",
		"guides-clean/alarm-rt/alarm.txt":  "TITLE: a\n",
		"devguide-clean/a.txt":             "TITLE: a\n",
		"devguide-clean/sub/c.txt":         "TITLE: c\n",
		"indexes/devguide-toc.json":        `{"sections": [{"title": "T"}]}`,
		"indexes/guides-index.json":        `{"Alpha": {"toc": true, "pages": 3}, "Beta": {"toc": ""}, "alarm-rt": {}}`,
		"indexes/source-index.json":        `{"X": {"path": "../devguide-rc/X.java", "module": "dev-rt"}, "A": {"path": "javax/A.java"}, "Y": {"path": "../devguide-rc/./Y.java"}}`,
		"indexes/class-index.json":         `{"BObject": {"package": "javax.baja.sys"}, "BX": {"package": "cl.alpha"}, "BDev": {"package": "javax.baja.dev"}, "BAbsent": {"package": "javax.baja.other", "others": [{"package": "javax.baja.sys"}]}}`,
		"indexes/jdk-class-index.json":     `{"Object": {"package": "java.lang"}}`,
	}
	for rel, text := range files {
		testkit.WriteFile(t, filepath.Join(base, rel), []byte(text))
	}
	return base
}

// covRepo writes a repository whose registry names vid with the install
// and the data root, and returns its path.
func covRepo(t *testing.T, dir, vid, install, base string) string {
	t.Helper()
	repo := filepath.Join(dir, "repo")
	reg := fmt.Sprintf(`{"versions": {%q: {"label": "x", "install_dir": %q, "data_root": %q}, "9.9.9.9": {"label": "y"}}}`,
		vid, install, base)
	testkit.WriteFile(t, filepath.Join(repo, "versions", "registry.json"), []byte(reg))
	return repo
}
