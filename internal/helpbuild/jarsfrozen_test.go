package helpbuild

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/angeles725/niagara-go/internal/helpbuild/internal/testkit"
	"github.com/angeles725/niagara-go/internal/helpbuild/pyfs"
	"github.com/angeles725/niagara-go/internal/pyjson"
)

// The jar listing and the install rules (S1) on synthetic jars, against
// their frozen Python output (frozen_test.go): every entry's classification,
// the listing-cache path of odd install names, and the glob rules.

// goClassify is the classification of every entry of jars: the listing,
// then per entry its doc-like, extract, exclude and course rules, the guide,
// licence and source-archive tests and its clean text name.
func goClassify(t *testing.T, srcDir string, jars []string) []string {
	t.Helper()
	rules := mustDefault(t)
	var out []string
	for _, jar := range jars {
		name := filepath.Base(jar)
		l := ReadListing(jar)
		out = append(out, pyjson.Dumps(testkit.Array(testkit.Str("jar"), testkit.Str(name), l.value()), pyfs.DumpCompact))
		if l == nil {
			continue
		}
		for _, e := range l.Entries {
			exclude := pyjson.Value{}
			if r := rules.ExcludeRule(name, e.Name); r != nil {
				exclude = testkit.Array(testkit.Str(r.ID), testkit.Str(r.Reason))
			}
			course := ""
			if r := rules.ExcludeRule("(course)", e.Name); r != nil {
				course = r.ID
			}
			extract := ""
			if r := rules.ExtractRule(name, e.Name); r != nil {
				extract = r.ID
			}
			out = append(out, pyjson.Dumps(testkit.Array(testkit.Str("entry"), testkit.Str(e.Name),
				testkit.Boolean(rules.IsDocLike(e.Name)), testkit.OptStr(extract), exclude, testkit.OptStr(course),
				testkit.OptStr(rules.CourseKind(e.Name)), testkit.Boolean(isGuidePage(e.Name)),
				testkit.Boolean(isLicence(e.Name)), testkit.Boolean(isSourceArchive("guides/"+e.Name)),
				testkit.Str(cleanTextName(e.Name, srcDir))), pyfs.DumpDefault))
		}
	}
	return out
}

// syntheticJars mirrors test_doc_jar_completeness / test_install_sweep
// fixtures plus the central-directory corners: duplicate names,
// directories, odd UTF-8 names, empty entries, raw cp437 names, extra
// fields, prepended data and unreadable files.
func syntheticJars(t *testing.T, dir string) []string {
	t.Helper()
	page := "<html><head><title>T</title></head><body><p>x</p></body></html>"
	jars := map[string][][2]string{
		"docClHVACTest-doc.jar": {{"doc/toc.xml", "<toc/>"}, {"doc/index.html", page}, {"doc/sub/", ""},
			{"doc/Tasks.html", page}, {"doc/sub/Overview.html", page}, {"doc/README.txt", "readme quokka\n"},
			{"doc/Tasks.html", "duplicate"}, {"doc/empty.txt", ""}, {"doc/cl/hvac/BAbsolute.bajadoc", "<x/>"},
			{"META-INF/MANIFEST.MF", "Manifest-Version: 1.0\n"}},
		"docDeveloper-ux.jar": {{"examples/bajaux/ZorbWidget.js", "define();"}, {"examples/bajaux/ZorbWidget.hbs", "<div/>"},
			{"examples/bajaux/ZorbStyle.css", ".z{}"}, {"rc/Playground.built.min.js", "var a;"},
			{"ext/@babel/standalone/babel.min.js", "var b;"}, {"com/tridium/docdeveloper/ux/Ex.class", "\xca\xfe"},
			{"docDeveloper-ux.lexicon", "k=v\n"}, {"META-INF/MANIFEST.MF", "M\n"}},
		"ascHelp.jar": {{"doc/About-Asc.html", page}, {"doc/style.css", "p{}"}, {"doc/lgpl-2.1.txt", "GNU\n"},
			{"doc/libfoo-1.0.tar.xz", "\xfd7zXZ"}, {"com/acme/asc/BAsc.class", "\xca\xfe"},
			{"com/acme/asc/icons/asc.png", "\x89PNG"}, {"com/acme/asc/wizard.bog", "<b/>"}, {"doc/About.htm", page},
			{"doc/readme.MD", "# r\n"}},
		"thing-ux.jar": {{"rc/notes.md", "# Notes\n"}, {"rc/node_modules/pkg/README.md", "npm\n"}, {"rc/readme.txt", "r\n"},
			{"rc/env/x.md", "x\n"}, {"rc/vendor/a.md", "v\n"}, {"rc/html/altHelp.html", page}, {"rc/html/page.html", page},
			{"help.html", page}, {"jsdoc/a/b.html", page}, {"org/apache/words.txt", "a\n"}, {"javax/baja/x.txt", "x"},
			{"javax/swing/x.txt", "x"}, {"version.txt", "1"}, {"about.html", page}, {"rc/a.css", "a{}"},
			{"x/package.html", page}, {"lib/ace/a.js", "a"}, {"rc/thing.js", "var x;"}},
		"workbench-wb.jar":         {{"com/tridium/workbench/media/about.html", page}, {"res/x/home.html", page}},
		"galileoSupervisor-wb.jar": {{"res/alertonHome.html", page}, {"res/x/home.html", page}},
		"course-files.jar": {{"text/a.txt", "a"}, {"text/sub/b.txt", "b"}, {"course/Labs/x/A.java", "class A {}"},
			{"course/Slide Code/y/src/doc/a.html", page}, {"course/a.pdf", "%PDF"}, {"solutions/x.java", "x"},
			{"niagara-4.15/dev/a/readme.txt", "r"}, {"niagara-4.15/dev/a/README.md", "r"}, {"x.pem", "k"},
			{"build.gradle.kts", "k"}, {"a.properties", "p"}, {"niagara-4.15/dev/m/src/x.js", "x"}},
		"odd-names.jar": {{"doc/Ünïcode ñ.html", page}, {"doc/日本語.md", "md"}, {"META-\u0130NF/x", "x"},
			{"doc/l\u0131cense.txt", "l"}, {"doc/\u212aelvin.Html", page}, {"doc/\u03a3.TXT", "s"}, {"DOC/X.HTM", page},
			{"doc/a b/c.md", "c"}, {"doc/tab\tname.txt", "t"}, {"doc/new\nline.html", page}, {"doc/Page.HTML", page},
			{"doc/Page.txt", "p"}, {"doc/changes.txt", "c"}, {"doc/emoji\U0001F600.txt", "e"},
			{"j\u017fdoc/a.html", page}, {"LICENSE", "l"}, {"doc/COPYING.txt", "c"}},
	}
	var paths []string
	for _, name := range []string{"docClHVACTest-doc.jar", "docDeveloper-ux.jar", "ascHelp.jar", "thing-ux.jar",
		"workbench-wb.jar", "galileoSupervisor-wb.jar", "course-files.jar", "odd-names.jar"} {
		p := filepath.Join(dir, name)
		testkit.WriteJar(t, p, jars[name]...)
		paths = append(paths, p)
	}
	var high strings.Builder
	for b := 0x80; b <= 0xff; b++ {
		high.WriteByte(byte(b))
	}
	ok := testkit.RawEntry{Name: "doc/a.html", Data: "a"}
	raw := []struct {
		name string
		data []byte
	}{
		{"cp437.jar", testkit.RawZip("", []testkit.RawEntry{{Name: "cp437/" + high.String() + ".txt", Data: "x"}, ok}, "")},
		{"nul.jar", testkit.RawZip("", []testkit.RawEntry{{Name: "doc/a.html\x00.exe", Data: "x"}}, "")},
		{"badutf8flag.jar", testkit.RawZip("", []testkit.RawEntry{{Name: "doc/\xff.html", Flags: 0x800}}, "")},
		{"unicodepath.jar", testkit.RawZip("", []testkit.RawEntry{{Name: "doc/x.html", Extra: testkit.UnicodePath("doc/x.html", "doc/ñ.html")},
			{Name: "doc/y.html", Extra: testkit.UnicodePath("other", "doc/ñ.html")}, {Name: "doc/z.html", Extra: testkit.UnicodePath("doc/z.html", "")}}, "")},
		{"badunicodepath.jar", testkit.RawZip("", []testkit.RawEntry{{Name: "doc/x.html", Extra: testkit.UnicodePath("doc/x.html", "\xff")}}, "")},
		{"zip64.jar", testkit.RawZip("", []testkit.RawEntry{{Name: "big.txt", Data: "12345", Size64: true, Extra: testkit.Zip64Size(5)}}, "")},
		{"shortzip64.jar", testkit.RawZip("", []testkit.RawEntry{{Name: "big.txt", Size64: true, Extra: testkit.ExtraField(1, []byte{1, 2})}}, "")},
		{"badextra.jar", testkit.RawZip("", []testkit.RawEntry{{Name: "a.txt", Extra: []byte{0x99, 0x99, 0x10, 0}}}, "")},
		{"prepended.jar", testkit.RawZip(strings.Repeat("#", 100), []testkit.RawEntry{ok}, "")},
		{"comment.jar", testkit.RawZip("", []testkit.RawEntry{ok}, "a comment")},
		{"zip64eocd.jar", testkit.Zip64Zip("", []testkit.RawEntry{ok, {Name: "doc/b.html", Data: "b"}}, 1, 0)},
		{"zip64prepended.jar", testkit.Zip64Zip(strings.Repeat("#", 50), []testkit.RawEntry{ok}, 1, 0)},
		{"zip64disks.jar", testkit.Zip64Zip("", []testkit.RawEntry{ok}, 2, 0)},
		{"zip64reloffpast.jar", testkit.Zip64Zip("", []testkit.RawEntry{ok}, 1, 1000)},
		{"zip64reloffshort.jar", testkit.Zip64Zip("", []testkit.RawEntry{ok}, 1, -1)},
		{"countone.jar", testkit.WithEntryCount(testkit.RawZip("", []testkit.RawEntry{ok, {Name: "doc/b.txt", Data: "b"}, {Name: "doc/c/", Data: ""},
			{Name: "doc/d.txt", Data: "d"}}, ""), 1)},
		{"notazip.jar", []byte("plain text, no zip at all")},
		{"empty.jar", nil},
		{"truncated.jar", testkit.RawZip("", []testkit.RawEntry{ok}, "")[:40]},
	}
	for _, r := range raw {
		p := filepath.Join(dir, r.name)
		testkit.WriteFile(t, p, r.data)
		paths = append(paths, p)
	}
	d := filepath.Join(dir, "dir.jar")
	if err := os.Mkdir(d, 0o755); err != nil {
		t.Fatal(err)
	}
	return append(paths, d)
}

// siblingSrc is a source tree for clean_text_name's X.html sibling check.
func siblingSrc(t *testing.T) string {
	t.Helper()
	src := filepath.Join(t.TempDir(), "src")
	for _, rel := range []string{"doc/Page.HTML", "doc/changes.html", "rc/readme.html"} {
		testkit.WriteFile(t, filepath.Join(src, filepath.FromSlash(rel)), []byte("<p/>"))
	}
	return src
}

// TestClassifyFrozen: the classification of every entry of the synthetic
// jars, with a source tree for clean_text_name's X.html sibling check.
func TestClassifyFrozen(t *testing.T) {
	testkit.FrozenOnly(t)
	jars := syntheticJars(t, t.TempDir())
	testkit.CheckFrozen(t, "jars/classify", goClassify(t, siblingSrc(t), jars))
}

// TestCachePathFrozen: the listing-cache path of installs named in odd
// ways (absolute, relative, doubled and trailing separators, non-ASCII,
// ".", UNC). The path names a hash of the absolute install path, so the
// test runs in "/" for the relative installs to resolve the same anywhere.
func TestCachePathFrozen(t *testing.T) {
	testkit.FrozenOnly(t)
	repo := t.TempDir()
	t.Chdir("/")
	var got []string
	for _, in := range []string{"/srv/niagara/install", "rel/../install", "/a//b/", "/opt/niagara ñ/4.15", ".", "//net/share"} {
		p, err := cachePath(repo, in)
		if err != nil {
			t.Fatal(err)
		}
		got = append(got, pyjson.Dumps(testkit.Str(p), pyfs.DumpDefault))
	}
	testkit.CheckFrozen(t, "jars/cache-path", testkit.Relabeled(got, repo, "<repo>"))
}

// TestGlobFrozen: the glob rules (GlobRe) on every pattern/path pair.
func TestGlobFrozen(t *testing.T) {
	pairs := [][2]string{}
	patterns := []string{"**/node_modules/**", "rc/**.md", "rc/*.md", "res/*Home.html", "*.jar", "doc/x.html",
		"META-INF/**", "k.jar", "jsdoc/**.html", "a?c", "**", "a+b(c)", "**/", "x**y", "course/Slide Code/**.java",
		"**/src/doc/*.html", "?", "*"}
	paths := []string{"node_modules/a.md", "rc/node_modules/a/b", "rc/env/x.md", "rc/x.md", "res/alertonHome.html",
		"sw/x.jar", "x.jar", "doc/x.htmlx", "doc/x.html", "META-\u0130NF/x", "meta-\u0131nf/x", "\u212a.jar",
		"j\u017fdoc/a.html", "a/c", "a\nc", "a\nb", "a+b(c)", "", "a/", "xy", "x/a/y", "COURSE/slide code/a/B.JAVA",
		"m/src/doc/a.html", "src/doc/a.html", "\u00df", "\U0001F600"}
	for _, p := range patterns {
		for _, s := range paths {
			pairs = append(pairs, [2]string{p, s})
		}
	}
	var got []pyjson.Value
	for _, pr := range pairs {
		got = append(got, testkit.Boolean(globRe(pr[0]).MatchString(pr[1])))
	}
	testkit.CheckFrozen(t, "jars/glob", []string{pyjson.Dumps(testkit.Array(got...), pyfs.DumpDefault)})
}
