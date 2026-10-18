package helpbuild

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/angeles725/niagara-go/internal/pyjson"
	"github.com/angeles725/niagara-go/internal/pyxml"
)

// checkS8Golden runs the Go builder of mode on a copy of testdata/s8/root/
// and compares its lines with testdata/s8/golden/<mode>.stdout and the
// named indexes with testdata/s8/golden/indexes/ (Python's output, see
// indexdriver.py); no Python is needed.
func checkS8Golden(t *testing.T, mode string, indexes ...string) {
	t.Helper()
	base := t.TempDir()
	copyTree(t, filepath.Join("testdata", "s8", "root"), base)
	res, err := goIndexBuild(mode, base)
	if err != nil {
		t.Fatal(err)
	}
	// The golden lines are a POSIX run's: the host separator prints as "/".
	var lines []string
	for _, l := range res.Log {
		l = strings.ReplaceAll(strings.ReplaceAll(l, base, "<base>"), string(filepath.Separator), "/")
		lines = append(lines, dumpLine(str("out"), str(l)))
	}
	lines = append(lines, dumpLine(str("exit"), intV(res.Status)))
	want, err := os.ReadFile(filepath.Join("testdata", "s8", "golden", mode+".stdout"))
	if err != nil {
		t.Fatal(err)
	}
	diffLines(t, mode+" stdout", strings.Split(strings.TrimSuffix(string(want), "\n"), "\n"), lines)
	got := fileTexts(t, filepath.Join(base, "indexes"))
	if len(got) != len(indexes) {
		t.Errorf("indexes/ holds %d files, want %d (%q)", len(got), len(indexes), indexes)
	}
	for _, name := range indexes {
		golden, err := os.ReadFile(filepath.Join("testdata", "s8", "golden", "indexes", name))
		if err != nil {
			t.Fatal(err)
		}
		if got[name] != string(golden) {
			t.Errorf("%s differs:\n go     %s\n python %s", name, around(got[name], string(golden)), around(string(golden), got[name]))
		}
	}
}

// The fixture: javadoc list (spans, CRLF, a byte that is not UTF-8, a
// duplicate, a non-class title), .bajadoc classes under devguide/ and
// guides/ (modules and profiles, every kind, a class without a qualified
// name, nested and namespaced classes, a file fixed by sanitizing, one
// that stays broken, Latin-1, names sharing a simple name) and jdk/.
func TestBuildClassIndexFixture(t *testing.T) {
	checkS8Golden(t, "class", "class-index.json", "jdk-class-index.json")
}

func TestBuildClassIndexParseFailures(t *testing.T) {
	base := t.TempDir()
	copyTree(t, filepath.Join("testdata", "s8", "root"), base)
	res, err := BuildClassIndex(base)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{filepath.Join(base, "devguide", "control", "com", "tridium", "control", "BHopeless.bajadoc")}
	if strings.Join(res.ParseFailures, "\n") != strings.Join(want, "\n") {
		t.Errorf("parse failures %q, want %q", res.ParseFailures, want)
	}
}

// _rank: javax.baja.* first, then javadoc pages before sibling-tree
// files, then package, then path; the sort is stable.
func TestMergeClassesRanking(t *testing.T) {
	entries := []namedClass{
		{"BX", classEntry{Path: "../devguide/z/BX.bajadoc", Package: "com.z", Type: "class", Module: "z"}},
		{"BX", classEntry{Path: "../guides/a/BX.bajadoc", Package: "com.a", Type: "class"}},
		{"BX", classEntry{Path: "../devguide/b/BX.bajadoc", Package: "javax.baja.b", Type: "class", Module: "b"}},
		{"BX", classEntry{Path: "javax/baja/a/BX.html", Package: "javax.baja.a", Type: "interface"}},
		{"BX", classEntry{Path: "com/y/BX.html", Package: "com.y", Type: "class"}},
		{"BX", classEntry{Path: "../devguide/a2/BX.bajadoc", Package: "javax.baja.a", Type: "interface", Module: "a-rt"}},
		{"Alone", classEntry{Path: "../jdk/Alone.bajadoc", Package: "", Type: "enum"}},
	}
	got := pyjson.Dumps(mergeClasses(entries), pyjson.Canonical)
	want := `{"Alone":{"package":"","path":"../jdk/Alone.bajadoc","type":"enum"},` +
		`"BX":{"module":"a-rt","others":[` +
		`{"module":"b","package":"javax.baja.b","path":"../devguide/b/BX.bajadoc","type":"class"},` +
		`{"package":"com.y","path":"com/y/BX.html","type":"class"},` +
		`{"package":"com.a","path":"../guides/a/BX.bajadoc","type":"class"},` +
		`{"module":"z","package":"com.z","path":"../devguide/z/BX.bajadoc","type":"class"}],` +
		`"package":"javax.baja.a","path":"javax/baja/a/BX.html","type":"interface"}}`
	if got != want {
		t.Errorf("merge:\n got %s\nwant %s", got, want)
	}
}

// _sanitize keeps references to XML characters and replaces the others
// (NUL, other C0 controls, surrogates, U+FFFE/U+FFFF, beyond U+10FFFF,
// values beyond any integer type) with "?".
func TestSanitizeCharRefs(t *testing.T) {
	for _, c := range []struct{ in, want string }{
		{"&#x0;&#0;&#x9;&#10;&#13;&#x1F;", "??&#x9;&#10;&#13;?"},
		{"&#x41;&#00000065;&#x0000041;", "&#x41;&#00000065;&#x0000041;"},
		{"&#xD800;&#xDFFF;&#xD7FF;&#xE000;", "??&#xD7FF;&#xE000;"},
		{"&#xFFFE;&#xFFFF;&#xFFFD;&#x10000;&#x10FFFF;&#x110000;", "??&#xFFFD;&#x10000;&#x10FFFF;?"},
		{"&#99999999999999999999;&#xFFFFFFFFFFFFFFFFFF;", "??"},
		{"&#X0;&#x;&#;&amp;&#x0", "&#X0;&#x;&#;&amp;&#x0"},
	} {
		if got := string(sanitizeCharRefs([]byte(c.in))); got != c.want {
			t.Errorf("%q: %q, want %q", c.in, got, c.want)
		}
	}
}

// bajadoc_xml._kind: enum, then annotation, then interface; module gets
// "-<runtimeProfile>" when the profile is not empty.
func TestClassHead(t *testing.T) {
	for _, c := range []struct {
		attrs string
		want  bajadocClassHead
	}{
		{``, bajadocClassHead{Kind: "class"}},
		{`name="A" packageName="p" module="m" runtimeProfile="ux"`, bajadocClassHead{"A", "p", "m-ux", "class"}},
		{`module="m" runtimeProfile=""`, bajadocClassHead{Module: "m", Kind: "class"}},
		{`runtimeProfile="rt"`, bajadocClassHead{Module: "-rt", Kind: "class"}},
		{`interface="true" annotation="true" enum="true"`, bajadocClassHead{Kind: "enum"}},
		{`interface="true" category="annotation"`, bajadocClassHead{Kind: "annotation_type"}},
		{`interface="true" category="enum"`, bajadocClassHead{Kind: "enum"}},
		{`interface="TRUE"`, bajadocClassHead{Kind: "class"}},
		{`interface="true"`, bajadocClassHead{Kind: "interface"}},
	} {
		root, err := pyxml.Parse([]byte(`<c `+c.attrs+`/>`), pyxml.Mode{ETree: true})
		if err != nil {
			t.Fatal(err)
		}
		if got := classHead(root); got != c.want {
			t.Errorf("%s: %+v, want %+v", c.attrs, got, c.want)
		}
	}
}
