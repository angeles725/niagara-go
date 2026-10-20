package helpbuild

import (
	"flag"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/angeles725/niagara-go/internal/helpbuild/internal/testkit"
)

// The S9 index builders (source-index.json, slots-index.json,
// xref-index.json) on the checked-in fixture against testdata/s9/golden/,
// which holds the Python builders' output (frozen when D47 retired the
// Python reference). -update-s9 rewrites it from the Go builders; review the
// diff before committing.

var updateS9 = flag.Bool("update-s9", false, "rewrite testdata/s9/golden/ from the Go builders (review the diff)")

// s9Builders are the S9 driver modes and their Go builders.
var s9Builders = map[string]func(string) (*IndexResult, error){
	"source": BuildSourceIndex,
	"slots":  BuildSlotsIndex,
	"xref":   BuildXrefIndex,
}

// s9Modes are the S9 driver modes, in the order rebuild-all runs them.
var s9Modes = []string{"source", "slots", "xref"}

// s9Indexes are the files each mode writes.
var s9Indexes = map[string]string{"source": "source-index.json", "slots": "slots-index.json", "xref": "xref-index.json"}

// checkS9Fixture fails when testdata/s9/root/ could make a golden file
// depend on the directory order of the file system: source_indexer lists
// the files of a folder unsorted (at most one .java per folder) and
// slots_indexer everything inside a module (at most one annotated .java
// per module). Order-dependent cases have no golden file (s9Corners).
func checkS9Fixture(t *testing.T) {
	t.Helper()
	perDir, annotated := map[string]int{}, map[string]int{}
	src := filepath.Join("testdata", "s9", "root")
	err := filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(p, ".java") {
			return err
		}
		perDir[filepath.Dir(p)]++
		rel, _ := filepath.Rel(filepath.Join(src, "source"), p)
		data, err := os.ReadFile(p)
		if module, _, ok := strings.Cut(filepath.ToSlash(rel), "/"); ok && !strings.HasPrefix(rel, "..") &&
			strings.Contains(string(data), "@Niagara") {
			annotated[module]++
		}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	for dir, n := range perDir {
		if n > 1 {
			t.Errorf("%s holds %d .java files: their order in source-index.json is the file system's (move them to s9Corners)", dir, n)
		}
	}
	for module, n := range annotated {
		if n > 1 {
			t.Errorf("source/%s holds %d annotated files: their order in slots-index.json is the file system's (move them to s9Corners)", module, n)
		}
	}
}

// checkS9Golden runs the Go builder of mode on a copy of testdata/s9/root/
// and compares its lines with testdata/s9/golden/<mode>.stdout and its
// index with testdata/s9/golden/indexes/, byte for byte (the fixture's
// output does not depend on directory order, see checkS9Fixture);
// -update-s9 rewrites both from the Go builder.
func checkS9Golden(t *testing.T, mode string) {
	t.Helper()
	checkS9Fixture(t)
	base := t.TempDir()
	testkit.CopyTree(t, filepath.Join("testdata", "s9", "root"), base)
	lines, _ := goIndexRun(mode, base)
	// The golden lines are a POSIX run's: the host separator prints as "/".
	for i, l := range lines {
		lines[i] = strings.ReplaceAll(strings.ReplaceAll(l, base, "<base>"), string(filepath.Separator), "/")
	}
	golden := filepath.Join("testdata", "s9", "golden")
	name := s9Indexes[mode]
	got := testkit.FileTexts(t, filepath.Join(base, "indexes"))
	if *updateS9 {
		testkit.WriteFile(t, filepath.Join(golden, mode+".stdout"), []byte(strings.Join(lines, "\n")+"\n"))
		testkit.WriteFile(t, filepath.Join(golden, "indexes", name), []byte(got[name]))
	}
	want, err := os.ReadFile(filepath.Join(golden, mode+".stdout"))
	if err != nil {
		t.Fatal(err)
	}
	testkit.DiffLines(t, mode+" stdout", strings.Split(strings.TrimSuffix(string(want), "\n"), "\n"), lines)
	if len(got) != 1 {
		t.Errorf("indexes/ holds %d files, want %s only", len(got), name)
	}
	data, err := os.ReadFile(filepath.Join(golden, "indexes", name))
	if err != nil {
		t.Fatal(err)
	}
	if g, w := got[name], string(data); g != w {
		t.Errorf("%s differs:\n go     %s\n golden %s", name, testkit.Around(g, w), testkit.Around(w, g))
	}
}

// TestBuildSourceIndexFixture: the Go source index of the fixture against
// the golden file (source/ modules with duplicate names, a package split
// over two lines, an indented package, a default package, a file named
// "X.java.java", non-ASCII names, devguide-rc/).
func TestBuildSourceIndexFixture(t *testing.T) {
	checkS9Golden(t, "source")
}

// TestBuildSlotsIndexFixture: the Go slots index of the fixture against
// the golden file (multi-line annotations, nested parentheses, literals
// holding commas, quotes and "=", comments inside and around annotations,
// unnamed and duplicate keys, a closer that does not match, slots merged
// from devguide/ and guides/ .bajadoc classes with and without a source
// entry, module or package).
func TestBuildSlotsIndexFixture(t *testing.T) {
	checkS9Golden(t, "slots")
}

// collision_key: three same-name classes keep three entries; when every
// key is taken the "#n" key is used and a warning printed
// (test_review_advisories.TestSlotsCollisionKey).
func TestSlotsCollisionKey(t *testing.T) {
	x := &slotsIndex{entries: map[string]*slotsEntry{}, res: &IndexResult{}}
	for _, pkg := range []string{"p1", "p2", "p3"} {
		x.put(x.collisionKey("BX", "mod", pkg), &slotsEntry{pkg: pkg})
	}
	if got := strings.Join(x.keys, "|"); got != "BX|BX (mod)|BX (p3)" {
		t.Errorf("keys %q", got)
	}
	for _, k := range []string{"BY", "BY (m)", "BY (p)", "BY (m, p)"} {
		x.put(k, &slotsEntry{})
	}
	if key := x.collisionKey("BY", "m", "p"); key != "BY (m, p) #2" || len(x.res.Stderr) != 1 ||
		!strings.HasPrefix(x.res.Stderr[0], "WARNING: slots for BY declared more than once") {
		t.Errorf("exhausted keys: %q, stderr %q", key, x.res.Stderr)
	}
	if key := x.collisionKey("BZ", "", ""); key != "BZ" {
		t.Errorf("free name: %q", key)
	}
	x.put("BZ", &slotsEntry{})
	if key := x.collisionKey("BZ", "", ""); key != "BZ ()" {
		t.Errorf("no qualifier: %q", key)
	}
}

// TestBuildXrefIndexFixture: the Go xref index of the fixture against the
// golden file (static, wildcard and self imports, imports of corpus types
// that are not B*/Niagara*, a class declared under another name than its
// file, a nested type, a default package that a same-name file with a
// package overrides, devguide-rc/).
func TestBuildXrefIndexFixture(t *testing.T) {
	checkS9Golden(t, "xref")
}
