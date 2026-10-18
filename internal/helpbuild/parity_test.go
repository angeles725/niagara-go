package helpbuild

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/angeles725/niagara-go/internal/pyjson"
)

// The Python-vs-Go parity harness: testdata/parity/driver.py runs the real
// niagara-help functions and prints JSON lines; the tests below rebuild
// every line from this package and require an empty diff. It needs python3
// and a niagara-help checkout (niagaraHelpTools: $NIAGARA_HELP_DIR, else
// found walking up from the test directory): plain `go test` skips visibly
// without them or with a python3 that is not CPython 3.15, a run that set a
// HELPBUILD_PARITY_* variable (make parity) fails (missingReference).

// parityDrivers are the Python sides of the parity tests.
var parityDrivers = []string{"driver.py", "htmldriver.py", "textdriver.py", "bajadocdriver.py"}

func driver(t *testing.T) (python, tools string) {
	t.Helper()
	python, err := exec.LookPath("python3")
	if err != nil {
		missingReference(t, "python3 not on PATH: parity not checked")
	}
	tools = niagaraHelpTools()
	if tools == "" {
		missingReference(t, "no niagara-help checkout ($NIAGARA_HELP_DIR unset, none under $NIAGARA_RESEARCH or walking up): parity not checked")
	}
	if _, err := os.Stat(filepath.Join(tools, "niagara_help_lib", "jar_listing.py")); err != nil {
		missingReference(t, fmt.Sprintf("niagara-help tools not available (%v): parity not checked", err))
	}
	for _, name := range parityDrivers {
		if _, err := os.Stat(filepath.Join("testdata", "parity", name)); err != nil {
			t.Fatalf("parity driver missing from the checkout: %v", err)
		}
	}
	v, err := pythonVersion(python)
	if err == nil {
		err = checkPythonMinor(v)
	}
	if err != nil {
		missingReference(t, fmt.Sprintf("parity driver interpreter %s: %v", python, err))
	}
	pythonLogged.Do(func() { t.Logf("parity driver python: CPython %s (%s)", v, python) })
	return python, tools
}

// parityPythonMinor is the CPython release the drivers' reference
// behaviour (html.parser above all) was pinned against.
const parityPythonMinor = "3.15"

var (
	pythonVersions sync.Map // interpreter path -> version string
	pythonLogged   sync.Once
)

// pythonVersion is the X.Y.Z of the interpreter at python, run once per
// path with the flags the drivers use.
func pythonVersion(python string) (string, error) {
	if v, ok := pythonVersions.Load(python); ok {
		return v.(string), nil
	}
	out, err := exec.Command(python, "-I", "-B", "-c",
		"import sys; print('%d.%d.%d' % sys.version_info[:3])").Output()
	if err != nil {
		return "", fmt.Errorf("cannot run it to read its version: %v", err)
	}
	v := strings.TrimSpace(string(out))
	pythonVersions.Store(python, v)
	return v, nil
}

// checkPythonMinor accepts a CPython parityPythonMinor.x version only.
func checkPythonMinor(v string) error {
	rest, ok := strings.CutPrefix(v, parityPythonMinor+".")
	if ok && rest != "" && strings.Trim(rest, "0123456789") == "" {
		return nil
	}
	return fmt.Errorf("it is CPython %q, the parity reference is CPython %s.x (html.parser and the other "+
		"stdlib behaviour the drivers compare change between minor releases): put a %s python3 first on PATH",
		v, parityPythonMinor, parityPythonMinor)
}

func runDriver(t *testing.T, stdin string, args ...string) []string {
	t.Helper()
	python, tools := driver(t)
	script, err := filepath.Abs(filepath.Join("testdata", "parity", "driver.py"))
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(python, append([]string{"-I", "-B", script, tools}, args...)...)
	cmd.Stdin = strings.NewReader(stdin)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("driver %v: %v\n%s", args, err, stderr.String())
	}
	return strings.Split(strings.TrimSuffix(string(out), "\n"), "\n")
}

func str(s string) pyjson.Value { return pyjson.String(s) }
func boolean(b bool) pyjson.Value {
	return pyjson.Value{Kind: pyjson.Bool, B: b}
}
func optStr(s string) pyjson.Value {
	if s == "" {
		return pyjson.Value{}
	}
	return str(s)
}
func array(vs ...pyjson.Value) pyjson.Value { return pyjson.Value{Kind: pyjson.Array, A: vs} }

// goClassify is driver.py classify rebuilt from this package.
func goClassify(t *testing.T, srcDir string, jars []string) []string {
	t.Helper()
	rules := mustDefault(t)
	var out []string
	for _, jar := range jars {
		name := filepath.Base(jar)
		l := ReadListing(jar)
		out = append(out, pyjson.Dumps(array(str("jar"), str(name), l.value()), DumpCompact))
		if l == nil {
			continue
		}
		for _, e := range l.Entries {
			exclude := pyjson.Value{}
			if r := rules.ExcludeRule(name, e.Name); r != nil {
				exclude = array(str(r.ID), str(r.Reason))
			}
			course := ""
			if r := rules.ExcludeRule("(course)", e.Name); r != nil {
				course = r.ID
			}
			extract := ""
			if r := rules.ExtractRule(name, e.Name); r != nil {
				extract = r.ID
			}
			out = append(out, pyjson.Dumps(array(str("entry"), str(e.Name),
				boolean(rules.IsDocLike(e.Name)), optStr(extract), exclude, optStr(course),
				optStr(rules.CourseKind(e.Name)), boolean(IsGuidePage(e.Name)),
				boolean(IsLicence(e.Name)), boolean(IsSourceArchive("guides/"+e.Name)),
				str(CleanTextName(e.Name, srcDir))), DumpDefault))
		}
	}
	return out
}

// around is a, cut to 160 bytes around its first difference from b (jar
// lines of a real install are megabytes long).
func around(a, b string) string {
	i := 0
	for i < len(a) && i < len(b) && a[i] == b[i] {
		i++
	}
	lo, hi := i-80, i+80
	if lo < 0 {
		lo = 0
	}
	if hi > len(a) {
		hi = len(a)
	}
	return fmt.Sprintf("...[%d]%q...", lo, a[lo:hi])
}

func diffLines(t *testing.T, what string, py, gol []string) {
	t.Helper()
	n := len(py)
	if len(gol) > n {
		n = len(gol)
	}
	bad := 0
	for i := 0; i < n; i++ {
		var p, g string
		if i < len(py) {
			p = py[i]
		}
		if i < len(gol) {
			g = gol[i]
		}
		if p != g {
			bad++
			if bad <= 10 {
				t.Errorf("%s line %d:\n python %s\n     go %s", what, i+1, around(p, g), around(g, p))
			}
		}
	}
	if bad > 0 {
		t.Fatalf("%s: %d of %d lines differ (python %d, go %d)", what, bad, n, len(py), len(gol))
	}
	t.Logf("%s: %d lines identical", what, n)
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
		writeJar(t, p, jars[name]...)
		paths = append(paths, p)
	}
	var high strings.Builder
	for b := 0x80; b <= 0xff; b++ {
		high.WriteByte(byte(b))
	}
	ok := rawEntry{name: "doc/a.html", data: "a"}
	raw := []struct {
		name string
		data []byte
	}{
		{"cp437.jar", rawZip("", []rawEntry{{name: "cp437/" + high.String() + ".txt", data: "x"}, ok}, "")},
		{"nul.jar", rawZip("", []rawEntry{{name: "doc/a.html\x00.exe", data: "x"}}, "")},
		{"badutf8flag.jar", rawZip("", []rawEntry{{name: "doc/\xff.html", flags: 0x800}}, "")},
		{"unicodepath.jar", rawZip("", []rawEntry{{name: "doc/x.html", extra: unicodePath("doc/x.html", "doc/ñ.html")},
			{name: "doc/y.html", extra: unicodePath("other", "doc/ñ.html")}, {name: "doc/z.html", extra: unicodePath("doc/z.html", "")}}, "")},
		{"badunicodepath.jar", rawZip("", []rawEntry{{name: "doc/x.html", extra: unicodePath("doc/x.html", "\xff")}}, "")},
		{"zip64.jar", rawZip("", []rawEntry{{name: "big.txt", data: "12345", size64: true, extra: zip64Size(5)}}, "")},
		{"shortzip64.jar", rawZip("", []rawEntry{{name: "big.txt", size64: true, extra: extraField(1, []byte{1, 2})}}, "")},
		{"badextra.jar", rawZip("", []rawEntry{{name: "a.txt", extra: []byte{0x99, 0x99, 0x10, 0}}}, "")},
		{"prepended.jar", rawZip(strings.Repeat("#", 100), []rawEntry{ok}, "")},
		{"comment.jar", rawZip("", []rawEntry{ok}, "a comment")},
		{"zip64eocd.jar", zip64Zip("", []rawEntry{ok, {name: "doc/b.html", data: "b"}}, 1, 0)},
		{"zip64prepended.jar", zip64Zip(strings.Repeat("#", 50), []rawEntry{ok}, 1, 0)},
		{"zip64disks.jar", zip64Zip("", []rawEntry{ok}, 2, 0)},
		{"zip64reloffpast.jar", zip64Zip("", []rawEntry{ok}, 1, 1000)},
		{"zip64reloffshort.jar", zip64Zip("", []rawEntry{ok}, 1, -1)},
		{"countone.jar", withEntryCount(rawZip("", []rawEntry{ok, {name: "doc/b.txt", data: "b"}, {name: "doc/c/", data: ""},
			{name: "doc/d.txt", data: "d"}}, ""), 1)},
		{"notazip.jar", []byte("plain text, no zip at all")},
		{"empty.jar", nil},
		{"truncated.jar", rawZip("", []rawEntry{ok}, "")[:40]},
	}
	for _, r := range raw {
		p := filepath.Join(dir, r.name)
		writeFile(t, p, r.data)
		paths = append(paths, p)
	}
	d := filepath.Join(dir, "dir.jar")
	if err := os.Mkdir(d, 0o755); err != nil {
		t.Fatal(err)
	}
	return append(paths, d)
}

// jarNamed picks a synthetic jar by base name, so adding a fixture never
// shifts which jar a test uses.
func jarNamed(t *testing.T, jars []string, name string) string {
	t.Helper()
	for _, j := range jars {
		if filepath.Base(j) == name {
			return j
		}
	}
	t.Fatalf("no synthetic jar %s", name)
	return ""
}

// siblingSrc is a source tree for clean_text_name's X.html sibling check.
func siblingSrc(t *testing.T) string {
	t.Helper()
	src := filepath.Join(t.TempDir(), "src")
	for _, rel := range []string{"doc/Page.HTML", "doc/changes.html", "rc/readme.html"} {
		writeFile(t, filepath.Join(src, filepath.FromSlash(rel)), []byte("<p/>"))
	}
	return src
}

func TestParitySyntheticJars(t *testing.T) {
	driver(t)
	jars := syntheticJars(t, t.TempDir())
	src := siblingSrc(t)
	py := runDriver(t, "", append([]string{"classify", src}, jars...)...)
	diffLines(t, "synthetic classify", py, goClassify(t, src, jars))
}

// defaultRealJars: five jars of different kinds from the EC-Net 4.3 install
// (the biggest doc jar, a small doc jar, an -rt, a -wb with an extract rule,
// a -ux with jsdoc/) and ffmpeg-wb.jar of 4.15, the one jar with a source
// archive.
func defaultRealJars() []string {
	m43 := "/mnt/c/Niagara/EC-Net4-4.3.58.18/modules/"
	return []string{m43 + "docDeveloper-doc.jar", m43 + "docLinuxN4-doc.jar", m43 + "alarm-rt.jar",
		m43 + "workbench-wb.jar", m43 + "batchJob-ux.jar", "/mnt/c/Niagara/Niagara-4.15.6.20/modules/ffmpeg-wb.jar"}
}

// realJars is the TestParityRealJars list: HELPBUILD_PARITY_JARS (path-list
// separated, empty entries dropped; explicit) else defaultRealJars. An
// explicit list with no jar is an error, never an empty comparison.
func realJars() (jars []string, explicit bool, err error) {
	env := os.Getenv("HELPBUILD_PARITY_JARS")
	if env == "" {
		return defaultRealJars(), false, nil
	}
	for _, j := range filepath.SplitList(env) {
		if j != "" {
			jars = append(jars, j)
		}
	}
	if len(jars) == 0 {
		return nil, true, fmt.Errorf("HELPBUILD_PARITY_JARS=%q names no jar: nothing to compare", env)
	}
	return jars, true, nil
}

func TestParityRealJars(t *testing.T) {
	needRealParity(t, "HELPBUILD_PARITY_JARS")
	driver(t)
	jars, explicit, err := realJars()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(jars[0]); err != nil && !explicit {
		t.Skipf("real install not mounted (%v): parity not checked", err)
	}
	for _, j := range jars {
		if _, err := os.Stat(j); err != nil {
			t.Fatalf("parity jar missing: %v", err)
		}
	}
	src := siblingSrc(t)
	py := runDriver(t, "", append([]string{"classify", src}, jars...)...)
	diffLines(t, "real classify", py, goClassify(t, src, jars))
}

func readBytes(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func goCache(t *testing.T, path string, jars ...string) int {
	t.Helper()
	ls := NewListings(path)
	for _, j := range jars {
		ls.Listing(j)
	}
	if err := ls.Save(); err != nil {
		t.Fatal(err)
	}
	return ls.Reads
}

// A cache Python writes is byte-identical to Go's, each reads the other's
// without opening a jar, and an update keeps Python's dict order.
func TestParityListingCache(t *testing.T) {
	driver(t)
	dir := t.TempDir()
	all := syntheticJars(t, filepath.Join(dir, "jars"))
	a, b, c, bad := jarNamed(t, all, "docClHVACTest-doc.jar"), jarNamed(t, all, "odd-names.jar"),
		jarNamed(t, all, "cp437.jar"), jarNamed(t, all, "notazip.jar")
	pyPath, goPath := filepath.Join(dir, "py", "c.json"), filepath.Join(dir, "go", "c.json")
	if got := runDriver(t, "", "cache", pyPath, a, b, bad); got[0] != "3" {
		t.Fatalf("python reads %q", got)
	}
	if n := goCache(t, goPath, a, b, bad); n != 3 {
		t.Fatalf("go reads %d", n)
	}
	if !bytes.Equal(readBytes(t, pyPath), readBytes(t, goPath)) {
		t.Fatalf("fresh caches differ:\npython %s\n    go %s", readBytes(t, pyPath), readBytes(t, goPath))
	}
	// Go's cross-read starts from a copy of the Python-written file, which it
	// must neither re-list nor rewrite.
	pyFresh := readBytes(t, pyPath)
	goRead := filepath.Join(dir, "go", "from-python.json")
	writeFile(t, goRead, pyFresh)
	if n := goCache(t, goRead, a, b, bad); n != 0 {
		t.Fatalf("go opened %d jars reading the Python cache", n)
	}
	if !bytes.Equal(readBytes(t, goRead), pyFresh) {
		t.Fatal("go rewrote the Python cache it only read")
	}
	if got := runDriver(t, "", "cache", goPath, a, b, bad); got[0] != "0" {
		t.Fatalf("python opened %s jars reading the Go cache", got[0])
	}
	pyNext, goNext := filepath.Join(dir, "py", "next.json"), filepath.Join(dir, "go", "next.json")
	writeFile(t, pyNext, pyFresh)
	writeFile(t, goNext, pyFresh)
	if got := runDriver(t, "", "cache", pyNext, c, a); got[0] != "1" {
		t.Fatalf("python update reads %q", got)
	}
	if n := goCache(t, goNext, c, a); n != 1 {
		t.Fatalf("go update reads %d", n)
	}
	if !bytes.Equal(readBytes(t, pyNext), readBytes(t, goNext)) {
		t.Fatalf("updated caches differ:\npython %s\n    go %s", readBytes(t, pyNext), readBytes(t, goNext))
	}
	t.Logf("cache files identical: %d and %d bytes", len(readBytes(t, pyPath)), len(readBytes(t, pyNext)))
}

func TestParityCachePath(t *testing.T) {
	driver(t)
	repo := t.TempDir()
	installs := []string{t.TempDir(), "rel/../install", "/a//b/", "/opt/niagara ñ/4.15", ".", "//net/share"}
	py := runDriver(t, "", append([]string{"path", repo}, installs...)...)
	var gol []string
	for _, in := range installs {
		p, err := CachePath(repo, in)
		if err != nil {
			t.Fatal(err)
		}
		gol = append(gol, pyjson.Dumps(str(p), DumpDefault))
	}
	diffLines(t, "cache_path", py, gol)
}

func TestParityGlob(t *testing.T) {
	driver(t)
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
	in, err := json.Marshal(pairs)
	if err != nil {
		t.Fatal(err)
	}
	py := runDriver(t, string(in), "glob")
	var got []pyjson.Value
	for _, pr := range pairs {
		got = append(got, boolean(GlobRe(pr[0]).MatchString(pr[1])))
	}
	diffLines(t, "glob_re", py, []string{pyjson.Dumps(array(got...), DumpDefault)})
	t.Logf("%d pattern/path pairs", len(pairs))
}
