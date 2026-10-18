package helpbuild

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/angeles725/niagara-go/internal/pyjson"
)

// Parity of the S9 index builders (source-index.json, slots-index.json,
// xref-index.json) through testdata/s9/indexdriver.py, with the S8
// machinery (indexBoth): each side runs on its own copy of the data root,
// then the printed lines (stdout, then stderr), the exit status and both
// whole trees are compared byte for byte. Both copies are made in the same
// order on one file system, so the directory order source_indexer and
// slots_indexer follow (os.walk without sorting) is the same on both sides.

var updateS9 = flag.Bool("update-s9", false, "rewrite testdata/s9/golden/ from the Python builders (TestParityS9Synthetic)")

// s9Builders are the S9 driver modes and their Go builders.
var s9Builders = map[string]func(string) (*IndexResult, error){
	"source": BuildSourceIndex,
}

// s9Modes are the S9 driver modes, in the order rebuild-all runs them.
var s9Modes = []string{"source"}

// s9Indexes are the files each mode writes.
var s9Indexes = map[string]string{"source": "source-index.json", "slots": "slots-index.json", "xref": "xref-index.json"}

// s9Bases copies testdata/s9/root/ twice.
func s9Bases(t *testing.T) (py, gol string) {
	t.Helper()
	py, gol = filepath.Join(t.TempDir(), "py"), filepath.Join(t.TempDir(), "go")
	for _, b := range []string{py, gol} {
		copyTree(t, filepath.Join("testdata", "s9", "root"), b)
	}
	return py, gol
}

// topSorted is an index with its top-level keys sorted (same formatting):
// the golden comparison of a builder whose key order follows the directory
// order of the file system the golden file was written on.
func topSorted(t *testing.T, text string) string {
	t.Helper()
	v, err := pyjson.Loads([]rune(text))
	if err != nil || v.Kind != pyjson.Object {
		return text
	}
	sort.Strings(v.Keys)
	return pyjson.Dumps(v, DumpIndent2Unicode)
}

// checkS9Golden runs the Go builder of mode on a copy of testdata/s9/root/
// and compares its lines with testdata/s9/golden/<mode>.stdout and its
// index with testdata/s9/golden/indexes/ (top-level key order aside, see
// topSorted); no Python is needed.
func checkS9Golden(t *testing.T, mode string) {
	t.Helper()
	base := t.TempDir()
	copyTree(t, filepath.Join("testdata", "s9", "root"), base)
	lines, _ := goIndexRun(mode, base)
	// The golden lines are a POSIX run's: the host separator prints as "/".
	for i, l := range lines {
		lines[i] = strings.ReplaceAll(strings.ReplaceAll(l, base, "<base>"), string(filepath.Separator), "/")
	}
	golden := filepath.Join("testdata", "s9", "golden")
	want, err := os.ReadFile(filepath.Join(golden, mode+".stdout"))
	if err != nil {
		t.Fatal(err)
	}
	diffLines(t, mode+" stdout", strings.Split(strings.TrimSuffix(string(want), "\n"), "\n"), lines)
	name := s9Indexes[mode]
	got := fileTexts(t, filepath.Join(base, "indexes"))
	if len(got) != 1 {
		t.Errorf("indexes/ holds %d files, want %s only", len(got), name)
	}
	data, err := os.ReadFile(filepath.Join(golden, "indexes", name))
	if err != nil {
		t.Fatal(err)
	}
	if g, w := topSorted(t, got[name]), topSorted(t, string(data)); g != w {
		t.Errorf("%s differs:\n go     %s\n python %s", name, around(g, w), around(w, g))
	}
}

// TestParityS9Synthetic: every builder on the checked-in fixture, then on
// the corners a fixture cannot hold (s9Corners), then on roots without the
// folders the builders read. It also re-checks the golden files (or
// rewrites them with -update-s9).
func TestParityS9Synthetic(t *testing.T) {
	driver(t)
	pyBase, goBase := s9Bases(t)
	for _, mode := range s9Modes {
		indexBoth(t, "fixture", mode, pyBase, goBase)
	}
	golden := filepath.Join("testdata", "s9", "golden")
	for _, mode := range s9Modes {
		lines, _ := runIndexDriver(t, mode, pyBase)
		text := strings.Join(relabelIndex(lines, pyBase), "\n") + "\n"
		index := fileTexts(t, filepath.Join(pyBase, "indexes"))[s9Indexes[mode]]
		if *updateS9 {
			writeFile(t, filepath.Join(golden, mode+".stdout"), []byte(text))
			writeFile(t, filepath.Join(golden, "indexes", s9Indexes[mode]), []byte(index))
		}
		if want, err := os.ReadFile(filepath.Join(golden, mode+".stdout")); err != nil || string(want) != text {
			t.Errorf("testdata/s9/golden/%s.stdout is not Python's output any more (%v): rerun with -update-s9", mode, err)
		}
		if want, err := os.ReadFile(filepath.Join(golden, "indexes", s9Indexes[mode])); err != nil ||
			topSorted(t, string(want)) != topSorted(t, index) {
			t.Errorf("testdata/s9/golden/indexes/%s is not Python's output any more (%v): rerun with -update-s9", s9Indexes[mode], err)
		}
	}
	for _, b := range []string{pyBase, goBase} {
		s9Corners(t, b)
	}
	for _, mode := range s9Modes {
		indexBoth(t, "corners", mode, pyBase, goBase)
	}
	for _, c := range []struct {
		what string
		keep []string
	}{
		{"empty root", nil},
		{"only devguide-rc", []string{"devguide-rc"}},
		{"source without devguide-rc", []string{"source", "devguide", "guides"}},
	} {
		pyB, goB := filepath.Join(t.TempDir(), "py"), filepath.Join(t.TempDir(), "go")
		for _, b := range []string{pyB, goB} {
			writeFile(t, filepath.Join(b, "readme.txt"), nil)
			for _, d := range c.keep {
				copyTree(t, filepath.Join("testdata", "s9", "root", d), filepath.Join(b, d))
			}
		}
		for _, mode := range s9Modes {
			indexBoth(t, c.what, mode, pyB, goB)
		}
	}
}

// s9Corners adds to a copy of the fixture what the checked-in fixture
// cannot hold: names whose order depends on the directory order of the
// file system (one class name in three folders of one module, five
// identical copies of a class), line ends and bytes git or Windows would
// change (CRLF, a lone CR, a form feed, U+2028, bytes that are not UTF-8),
// a string literal left open by a backslash at the end of a file, a
// backslash in a file name, and links: a linked module, a linked folder
// inside a module (listed, not entered), a linked .java file and a dangling
// one.
func s9Corners(t *testing.T, base string) {
	t.Helper()
	src := filepath.Join(base, "source")
	for i, pkg := range []string{"p1", "p2", "p3"} {
		dir := filepath.Join(src, "same-rt", pkg)
		writeFile(t, filepath.Join(dir, "BX.java"), fmt.Appendf(nil,
			"package %s;\n@NiagaraProperty(name = \"v%d\", type = \"int\")\npublic class BX {}\n", pkg, i))
	}
	for i := range 5 {
		writeFile(t, filepath.Join(src, "dup-rt", fmt.Sprintf("d%d", i), "BDup.java"),
			[]byte("package dup;\n@NiagaraAction(name = \"go\")\npublic class BDup {}\n"))
	}
	for name, data := range map[string]string{
		"Crlf.java":    "package crlf.pkg;\r\nimport javax.baja.sys.BObject;\r\n@NiagaraProperty(name = \"a\",\r\n type = \"int\")\r\npublic class Crlf\r\n{\r\n}\r\n",
		"LoneCr.java":  "package lone.cr;\rpublic class LoneCr {}\r@NiagaraTopic(name = \"t\", eventType = \"BValue\")\r",
		"Feed.java":    "package feed;\fpublic class Feed {}\n@NiagaraProperty(name = \"f\", type = \"int\")\n",
		"Sep.java":     "package sep; public class Sep {}\n",
		"Bad.java":     "package javax.baja.\xff\xfe;\nimport javax.baja.sys.B\xe9ad;\n@NiagaraProperty(name = \"caf\xe9\", type = \"int\")\npublic class Bad {}\n\xe2\x82",
		"Open.java":    "package open;\npublic class Open {}\n@NiagaraProperty(name = \"o\", type = \"String\", defaultValue = \"x\\",
		"Back\\S.java": "package back;\nclass BackS {}\n",
	} {
		writeFile(t, filepath.Join(src, "corner-rt", "c", name), []byte(data))
	}
	writeFile(t, filepath.Join(base, "elsewhere", "linked", "BLinked.java"),
		[]byte("package linked;\n@NiagaraProperty(name = \"l\", type = \"int\")\npublic class BLinked {}\n"))
	for link, target := range map[string]string{
		filepath.Join(src, "linked-rt"):                           filepath.Join(base, "elsewhere"),
		filepath.Join(src, "corner-rt", "c", "folder"):            filepath.Join(base, "elsewhere", "linked"),
		filepath.Join(src, "corner-rt", "c", "BFileLink.java"):    filepath.Join(base, "elsewhere", "linked", "BLinked.java"),
		filepath.Join(src, "corner-rt", "c", "BDangling.java"):    filepath.Join(base, "missing.java"),
		filepath.Join(base, "devguide-rc", "examples", "Rc.java"): filepath.Join(base, "elsewhere", "linked", "BLinked.java"),
	} {
		if err := os.Symlink(target, link); err != nil {
			t.Fatal(err)
		}
	}
}

// A .java file that cannot be read (skipped by source and slots, named
// after its file by xref): the lines and the index of each builder are
// compared (the trees cannot be read whole).
func TestParityS9Unreadable(t *testing.T) {
	driver(t)
	if os.Geteuid() == 0 {
		t.Skip("root reads a mode-000 file")
	}
	pyBase, goBase := s9Bases(t)
	for _, b := range []string{pyBase, goBase} {
		locked := filepath.Join(b, "source", "corner-rt", "BLocked.java")
		writeFile(t, locked, []byte("package locked;\n@NiagaraProperty(name = \"k\", type = \"int\")\npublic class BLocked {}\n"))
		if err := os.Chmod(locked, 0); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.Chmod(locked, 0o644) })
	}
	for _, mode := range s9Modes {
		pyLines, _ := runIndexDriver(t, mode, pyBase)
		goLines, _ := goIndexRun(mode, goBase)
		diffLines(t, "unreadable "+mode+" lines", relabelIndex(pyLines, pyBase), relabelIndex(goLines, goBase))
		name := s9Indexes[mode]
		py, _ := os.ReadFile(filepath.Join(pyBase, "indexes", name))
		gol, _ := os.ReadFile(filepath.Join(goBase, "indexes", name))
		if string(py) != string(gol) {
			t.Errorf("unreadable %s differs:\n go     %s\n python %s", name, around(string(gol), string(py)), around(string(py), string(gol)))
		}
	}
}

// s9Inputs are the folders of a data root the S9 builders read.
var s9Inputs = []string{"source", "devguide-rc", "devguide", "guides"}

// TestParityS9Real builds the S9 indexes of real versions on both sides
// (the versions of TestParityS8Real: HELPBUILD_PARITY_INDEXES, or
// 4.14.0.162 and 4.15.6.20 with the switches) and logs each file's size
// and both run times.
func TestParityS9Real(t *testing.T) {
	needRealParity(t, "HELPBUILD_PARITY_415", s8IndexesVar)
	driver(t)
	vids, err := s8Versions()
	if err != nil {
		t.Fatal(err)
	}
	for _, vid := range vids {
		t.Run(vid, func(t *testing.T) {
			pyBase, goBase := realIndexBase(t, vid, s9Inputs, "source"), realIndexBase(t, vid, s9Inputs, "source")
			for _, mode := range s9Modes {
				py, gol := indexBoth(t, vid, mode, pyBase, goBase)
				t.Logf("%s %s: python %.2fs, go %.2fs", vid, mode, py.Seconds(), gol.Seconds())
				if st, err := os.Stat(filepath.Join(goBase, "indexes", s9Indexes[mode])); err == nil {
					t.Logf("%s %s: %d bytes identical", vid, s9Indexes[mode], st.Size())
				}
			}
		})
	}
}

// TestBuildSourceIndexFixture: the Go source index of the fixture against
// the golden file (source/ modules with duplicate names, a package split
// over two lines, an indented package, a default package, a file named
// "X.java.java", non-ASCII names, devguide-rc/).
func TestBuildSourceIndexFixture(t *testing.T) {
	checkS9Golden(t, "source")
}
