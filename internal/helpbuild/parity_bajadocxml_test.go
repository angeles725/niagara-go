package helpbuild

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

// Parity of the S7b builder (.bajadoc XML -> bajadoc-clean/) through
// testdata/parity/bajadocdriver.py (mode xmlrun; the real runs do mode run,
// S7a, first, as the build does): each side runs on its own copy of the data
// root, then the printed lines of every step, the exit status and both
// whole trees are compared.

// goBajadocXMLRun is bajadocdriver.py xmlrun rebuilt from this package.
func goBajadocXMLRun(base string) ([]string, time.Duration) {
	start := time.Now()
	res, err := ParseBajadocXML(base)
	took := time.Since(start)
	var out []string
	for _, l := range res.Log {
		out = append(out, dumpLine(str("out"), str(l)))
	}
	if err != nil {
		return append(out, dumpLine(str("raise"))), took
	}
	return append(out, dumpLine(str("exit"), intV(0))), took
}

// bajadocStep is one builder run on both sides: the driver mode and its Go
// twin.
type bajadocStep struct {
	mode string
	gol  func(base string) ([]string, time.Duration)
}

var (
	stepS7a = bajadocStep{"run", goBajadocRun}
	stepS7b = bajadocStep{"xmlrun", goBajadocXMLRun}
)

// bajadocStepsBoth runs steps in order on both sides, compares the lines
// of each, then both whole trees; it logs and returns the run times per
// step. Each of after runs before the tree comparison.
func bajadocStepsBoth(t *testing.T, what, pyBase, goBase string, steps []bajadocStep, after ...func()) {
	t.Helper()
	var times string
	for _, s := range steps {
		pyLines, py := runBajadocDriver(t, s.mode, pyBase)
		goLines, gol := s.gol(goBase)
		diffLines(t, what+" "+s.mode+" lines", relabelBajadoc(pyLines, pyBase), relabelBajadoc(goLines, goBase))
		times += fmt.Sprintf(" %s python %.1fs go %.1fs;", s.mode, py.Seconds(), gol.Seconds())
	}
	for _, f := range after {
		f()
	}
	files, size := diffContents(t, pyBase, goBase)
	t.Logf("%s: %d files, %d bytes compared;%s", what, files, size, times)
	diffLines(t, what+" tree", treeLines(t, pyBase), treeLines(t, goBase))
}

// bajadocXMLCorners adds what a checked-in fixture cannot hold: a page left
// under the escaped generic name, S7a's page of a class with a javadoc
// page, a link to a .bajadoc file (read) and one to a folder (not entered).
func bajadocXMLCorners(t *testing.T, base string) {
	t.Helper()
	demo := filepath.Join(base, "bajadoc-clean", "javax", "baja", "demo")
	if runtime.GOOS != "windows" {
		writeFile(t, filepath.Join(demo, "Box<T>.txt"), []byte("old escaped name"))
	}
	writeFile(t, filepath.Join(demo, "BHasHtml.txt"), []byte("CLASS: BHasHtml\nfrom the javadoc page\n"))
	writeFile(t, filepath.Join(base, "extra", "BLinked.bajadoc"),
		[]byte(`<bajadoc><class qualifiedName="a.BLinked" name="BLinked" packageName="a"/></bajadoc>`))
	writeFile(t, filepath.Join(base, "extra", "dir", "BNotEntered.bajadoc"),
		[]byte(`<bajadoc><class qualifiedName="a.BNotEntered" name="BNotEntered" packageName="a"/></bajadoc>`))
	guides := filepath.Join(base, "guides")
	for link, target := range map[string]string{"Linked.bajadoc": "../extra/BLinked.bajadoc", "linked-dir": "../extra/dir"} {
		if err := os.Symlink(target, filepath.Join(guides, link)); err != nil {
			t.Fatal(err)
		}
	}
}

// TestParityBajadocXMLSynthetic: the checked-in fixture, the corners, a
// second run over the first one's pages, a .bajadoc file that cannot be
// read (dangling link, mode 000: both raise) and a data root without
// devguide/ or guides/. It also re-checks the golden files.
func TestParityBajadocXMLSynthetic(t *testing.T) {
	driver(t)
	pyBase, goBase := filepath.Join(t.TempDir(), "py"), filepath.Join(t.TempDir(), "go")
	for _, b := range []string{pyBase, goBase} {
		copyTree(t, filepath.Join("testdata", "bajadocxml", "root"), b)
	}
	only := []bajadocStep{stepS7b}
	bajadocStepsBoth(t, "fixture", pyBase, goBase, only)
	golden := fileTexts(t, filepath.Join("testdata", "bajadocxml", "clean"))
	if got := fileTexts(t, filepath.Join(pyBase, "bajadoc-clean")); fmt.Sprint(got) != fmt.Sprint(golden) {
		t.Errorf("testdata/bajadocxml/clean/ is not Python's output any more: regenerate it (bajadocdriver.py xmlrun)")
	}
	for _, b := range []string{pyBase, goBase} {
		bajadocXMLCorners(t, b)
	}
	bajadocStepsBoth(t, "corners", pyBase, goBase, only)
	bajadocStepsBoth(t, "second run", pyBase, goBase, only)

	for _, b := range []string{pyBase, goBase} {
		if err := os.Symlink("missing.bajadoc", filepath.Join(b, "devguide", "Dangling.bajadoc")); err != nil {
			t.Fatal(err)
		}
	}
	bajadocStepsBoth(t, "dangling .bajadoc link", pyBase, goBase, only)
	if os.Geteuid() != 0 && runtime.GOOS != "windows" {
		var unlock []func()
		for _, b := range []string{pyBase, goBase} {
			_ = os.Remove(filepath.Join(b, "devguide", "Dangling.bajadoc"))
			locked := filepath.Join(b, "guides", "vendor", "com", "vendor", "BVendor.bajadoc")
			if err := os.Chmod(locked, 0); err != nil {
				t.Fatal(err)
			}
			restore := func() { _ = os.Chmod(locked, 0o644) }
			t.Cleanup(restore)
			unlock = append(unlock, restore)
		}
		bajadocStepsBoth(t, "unreadable .bajadoc", pyBase, goBase, only, unlock...)
	}

	empty := []string{filepath.Join(t.TempDir(), "py"), filepath.Join(t.TempDir(), "go")}
	for _, b := range empty {
		writeFile(t, filepath.Join(b, "devguide-not", "x.bajadoc"), []byte(`<bajadoc/>`))
	}
	bajadocStepsBoth(t, "no devguide or guides", empty[0], empty[1], only)
}

// bajadocXMLVersions are the versions TestParityBajadocXMLReal builds:
// HELPBUILD_PARITY_BAJADOCXML (a path-list of version ids) when set, else
// 4.3.58.18 (XML only) and 4.14.0.162 with HELPBUILD_PARITY_REAL=1 and
// 4.15.6.20 with HELPBUILD_PARITY_415=1; none is an error.
func bajadocXMLVersions() ([]string, error) {
	return versionList("HELPBUILD_PARITY_BAJADOCXML",
		[][2]string{{realParityVar, "4.3.58.18"}, {realParityVar, "4.14.0.162"}, {"HELPBUILD_PARITY_415", "4.15.6.20"}})
}

// realBajadocXMLBase makes a temporary data root whose bajadoc/, devguide/
// and guides/ link to those of the registered data root of vid (only read;
// bajadoc-clean/ is written in the temporary root).
func realBajadocXMLBase(t *testing.T, vid string) string {
	t.Helper()
	data := registeredDataRoot(t, vid)
	base := t.TempDir()
	linked := 0
	for _, d := range []string{"bajadoc", "devguide", "guides"} {
		src := filepath.Join(data, d)
		if !isDir(src) {
			continue
		}
		if err := os.Symlink(src, filepath.Join(base, d)); err != nil {
			t.Fatal(err)
		}
		if d != "bajadoc" {
			linked++
		}
	}
	if linked == 0 {
		missingReference(t, fmt.Sprintf("version %s has no devguide/ or guides/ (%s)", vid, data))
	}
	return base
}

// TestParityBajadocXMLReal builds bajadoc-clean/ of real versions on both
// sides the way the build does, S7a then S7b, and logs the run times.
func TestParityBajadocXMLReal(t *testing.T) {
	needRealParity(t, "HELPBUILD_PARITY_415", "HELPBUILD_PARITY_BAJADOCXML")
	driver(t)
	vids, err := bajadocXMLVersions()
	if err != nil {
		t.Fatal(err)
	}
	for _, vid := range vids {
		t.Run(vid, func(t *testing.T) {
			pyBase, goBase := realBajadocXMLBase(t, vid), realBajadocXMLBase(t, vid)
			bajadocStepsBoth(t, vid, pyBase, goBase, []bajadocStep{stepS7a, stepS7b})
		})
	}
}
