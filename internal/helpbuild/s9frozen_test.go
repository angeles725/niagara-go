package helpbuild

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/angeles725/niagara-go/internal/helpbuild/internal/testkit"
)

// The S9 index builders (source-index.json, slots-index.json,
// xref-index.json) on the corners a checked-in fixture cannot hold, against
// their frozen Python output (frozen_test.go): the printed lines (stdout,
// then stderr; the root shown as <base>), the exit status and indexes/ after
// each builder, and the whole tree after the last.

// indexFrozen runs mode on base and checks its lines and what indexes/ then
// holds against testdata/frozen/<name>/<mode>.txt.
func indexFrozen(t *testing.T, name, mode, base string) {
	t.Helper()
	lines, _ := goIndexRun(mode, base)
	got := append(relabelIndex(lines, base), "indexes:")
	testkit.CheckFrozen(t, name+"/"+mode, append(got, testkit.TreeLines(t, filepath.Join(base, "indexes"))...))
}

// treeFrozen checks the whole tree of base, once every builder ran, against
// testdata/frozen/<name>/tree.txt.
func treeFrozen(t *testing.T, name, base string) {
	t.Helper()
	testkit.CheckFrozen(t, name+"/tree", testkit.TreeLines(t, base))
}

// lockFiles makes paths unreadable until the test ends; it skips t when the
// process reads mode-000 files anyway (root).
func lockFiles(t *testing.T, paths ...string) {
	t.Helper()
	if os.Geteuid() == 0 {
		t.Skip("root reads a mode-000 file")
	}
	for _, p := range paths {
		if err := os.Chmod(p, 0); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.Chmod(p, 0o644) })
	}
}

// TestS9CornersFrozen: every S9 builder on the checked-in fixture, then on
// the order-independent corners (s9Corners), then on roots without the
// folders the builders read.
func TestS9CornersFrozen(t *testing.T) {
	testkit.FrozenOnly(t)
	base := filepath.Join(t.TempDir(), "go")
	testkit.CopyTree(t, filepath.Join("testdata", "s9", "root"), base)
	for _, mode := range s9Modes { // the fixture's own output is the golden of checkS9Golden
		goIndexRun(mode, base)
	}
	s9Corners(t, base)
	for _, mode := range s9Modes {
		indexFrozen(t, "s9/corners", mode, base)
	}
	treeFrozen(t, "s9/corners", base)
	for _, c := range []struct {
		name  string
		keep  []string
		files map[string]string
	}{
		{"empty-root", nil, nil},
		{"only-devguide-rc", []string{"devguide-rc"}, nil},
		{"source-without-devguide-rc", []string{"source", "devguide", "guides"}, nil},
		{"import-ending-with-a-dot", nil, map[string]string{ // xref raised IndexError
			"source/m-rt/a/A.java": "package a;\nimport b.C;\npublic class A {}\n",
			"source/m-rt/b/B.java": "package b;\nimport c.;\npublic class B {}\n",
			"source/m-rt/c/C.java": "package c;\nimport a.A;\npublic class C {}\n",
		}},
		{"source-is-a-file", nil, map[string]string{"source": "", "devguide-rc/R.java": "class R {}\n"}},
	} {
		b := filepath.Join(t.TempDir(), "go")
		testkit.WriteFile(t, filepath.Join(b, "readme.txt"), nil)
		for _, d := range c.keep {
			testkit.CopyTree(t, filepath.Join("testdata", "s9", "root", d), filepath.Join(b, d))
		}
		for rel, data := range c.files {
			testkit.WriteFile(t, filepath.Join(b, filepath.FromSlash(rel)), []byte(data))
		}
		for _, mode := range s9Modes {
			indexFrozen(t, "s9/"+c.name, mode, b)
		}
		treeFrozen(t, "s9/"+c.name, b)
	}
}

// s9Corners adds to a copy of the fixture what the checked-in fixture
// cannot hold: line ends and bytes git or Windows would change (CRLF, a
// lone CR, a form feed, U+2028, bytes that are not UTF-8), a string
// literal left open by a backslash at the end of a file, a backslash in a
// file name, and links: a linked module, a linked folder inside a module
// (listed, not entered), a linked .java file and a dangling one. The
// parity test also held names whose order follows the directory order of
// the file system (one class in three folders of a module, five copies of
// a class); a frozen file cannot hold that order, so they are not here.
func s9Corners(t *testing.T, base string) {
	t.Helper()
	src := filepath.Join(base, "source")
	for name, data := range map[string]string{
		"Crlf.java":    "package crlf.pkg;\r\nimport javax.baja.sys.BObject;\r\n@NiagaraProperty(name = \"a\",\r\n type = \"int\")\r\npublic class Crlf\r\n{\r\n}\r\n",
		"LoneCr.java":  "package lone.cr;\rpublic class LoneCr {}\r@NiagaraTopic(name = \"t\", eventType = \"BValue\")\r",
		"Feed.java":    "package feed;\fpublic class Feed {}\n@NiagaraProperty(name = \"f\", type = \"int\")\n",
		"Sep.java":     "package sep;\u2028public class Sep {}\n",
		"Bad.java":     "package javax.baja.\xff\xfe;\nimport javax.baja.sys.B\xe9ad;\n@NiagaraProperty(name = \"caf\xe9\", type = \"int\")\npublic class Bad {}\n\xe2\x82",
		"Open.java":    "package open;\npublic class Open {}\n@NiagaraProperty(name = \"o\", type = \"String\", defaultValue = \"x\\",
		"Back\\S.java": "package back;\nclass BackS {}\n",
	} {
		testkit.WriteFile(t, filepath.Join(src, "corner-rt", "c", name), []byte(data))
	}
	testkit.WriteFile(t, filepath.Join(base, "elsewhere", "linked", "BLinked.java"),
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
// after its file by xref) and a .bajadoc that cannot be read (slots
// raised, nothing written): the lines and the index of each builder.
func TestS9UnreadableFrozen(t *testing.T) {
	testkit.FrozenOnly(t)
	base := filepath.Join(t.TempDir(), "go")
	testkit.CopyTree(t, filepath.Join("testdata", "s9", "root"), base)
	locked := filepath.Join(base, "source", "corner-rt", "BLocked.java")
	testkit.WriteFile(t, locked, []byte("package locked;\n@NiagaraProperty(name = \"k\", type = \"int\")\npublic class BLocked {}\n"))
	lockFiles(t, locked, filepath.Join(base, "guides", "clX", "Sample.bajadoc"))
	for _, mode := range s9Modes {
		lines, _ := goIndexRun(mode, base)
		index, err := os.ReadFile(filepath.Join(base, "indexes", s9Indexes[mode]))
		if err != nil {
			index = []byte("<none>")
		}
		testkit.CheckFrozen(t, "s9/unreadable/"+mode, append(relabelIndex(lines, base), "index:", string(index)))
	}
}
