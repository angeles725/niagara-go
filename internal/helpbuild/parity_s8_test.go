package helpbuild

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/angeles725/niagara-go/internal/pycompat"
)

// Parity of the S8 index builders (class-index.json + jdk-class-index.json,
// inheritance.json, method-index.json) through testdata/s8/indexdriver.py:
// each side runs on its own copy of the data root, then the printed lines
// (the root shown as <base>), the exit status and both whole trees are
// compared byte for byte.

var updateS8 = flag.Bool("update-s8", false, "rewrite testdata/s8/golden/ from the Python builders (TestParityS8Synthetic)")

// s8IndexesVar selects the versions of TestParityS8Real (a path-list of
// version ids); it is one of parityVars, so it counts as a parity request.
const s8IndexesVar = "HELPBUILD_PARITY_INDEXES"

// s8Modes are the driver modes, in the order rebuild-all runs the builders.
var s8Modes = []string{"class", "inherit", "method"}

// runIndexDriver runs the indexdriver.py of mode's slice (s8 or s9) and returns its lines (the "took" line
// removed) and the builder's own run time.
func runIndexDriver(t *testing.T, mode, base string) ([]string, time.Duration) {
	t.Helper()
	python, tools := driver(t)
	slice := "s8"
	if s9Builders[mode] != nil {
		slice = "s9"
	}
	script, err := filepath.Abs(filepath.Join("testdata", slice, "indexdriver.py"))
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(python, "-I", "-B", script, tools, mode, base)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("indexdriver %s: %v\n%s", mode, err, stderr.String())
	}
	lines := strings.Split(strings.TrimSuffix(string(out), "\n"), "\n")
	var took time.Duration
	if n := len(lines); n > 0 && strings.HasPrefix(lines[n-1], `["took", `) {
		secs, _ := strconv.ParseFloat(strings.TrimSuffix(strings.TrimPrefix(lines[n-1], `["took", `), "]"), 64)
		took, lines = time.Duration(secs*float64(time.Second)), lines[:n-1]
	}
	return lines, took
}

// goIndexBuild runs the Go builder of mode.
func goIndexBuild(mode, base string) (*IndexResult, error) {
	switch mode {
	case "class":
		return BuildClassIndex(base)
	case "inherit":
		return BuildInheritance(base)
	case "method":
		return BuildMethodIndex(base)
	}
	if build := s9Builders[mode]; build != nil {
		return build(base)
	}
	return nil, fmt.Errorf("unknown mode %s", mode)
}

// goIndexRun is indexdriver.py's mode rebuilt from this package.
func goIndexRun(mode, base string) ([]string, time.Duration) {
	start := time.Now()
	res, err := goIndexBuild(mode, base)
	took := time.Since(start)
	var out []string
	if res != nil {
		for _, l := range res.Log {
			out = append(out, dumpLine(str("out"), str(l)))
		}
		for _, l := range res.Stderr {
			out = append(out, dumpLine(str("err"), str(l)))
		}
	}
	if err != nil {
		return append(out, dumpLine(str("raise"))), took
	}
	return append(out, dumpLine(str("exit"), intV(res.Status))), took
}

// relabelIndex shows base as <base> and a raise without its class.
func relabelIndex(lines []string, base string) []string {
	out := make([]string, len(lines))
	for i, l := range lines {
		if strings.HasPrefix(l, `["raise"`) {
			l = `["raise"]`
		}
		out[i] = strings.ReplaceAll(l, base, "<base>")
	}
	return out
}

// indexBoth runs mode on both sides and compares lines and trees; it
// returns the Python and Go builder times.
func indexBoth(t *testing.T, what, mode, pyBase, goBase string) (py, gol time.Duration) {
	t.Helper()
	pyLines, py := runIndexDriver(t, mode, pyBase)
	goLines, gol := goIndexRun(mode, goBase)
	diffLines(t, what+" "+mode+" lines", relabelIndex(pyLines, pyBase), relabelIndex(goLines, goBase))
	files, size := diffContents(t, pyBase, goBase)
	t.Logf("%s %s: %d files, %d bytes compared", what, mode, files, size)
	diffLines(t, what+" "+mode+" tree", treeLines(t, pyBase), treeLines(t, goBase))
	return py, gol
}

// s8Bases copies testdata/s8/root/ twice.
func s8Bases(t *testing.T) (py, gol string) {
	t.Helper()
	py, gol = filepath.Join(t.TempDir(), "py"), filepath.Join(t.TempDir(), "go")
	for _, b := range []string{py, gol} {
		copyTree(t, filepath.Join("testdata", "s8", "root"), b)
	}
	return py, gol
}

// TestParityS8Synthetic: every builder on the checked-in fixture, then on
// the corners a fixture cannot hold (s8Corners, s8CleanCorners), and on roots without the
// folders each builder reads. It also re-checks the golden files (or
// rewrites them with -update-s8).
func TestParityS8Synthetic(t *testing.T) {
	driver(t)
	pyBase, goBase := s8Bases(t)
	golden := filepath.Join("testdata", "s8", "golden")
	for _, mode := range s8Modes {
		indexBoth(t, "fixture", mode, pyBase, goBase)
	}
	pyLog := map[string]string{}
	for _, mode := range s8Modes {
		lines, _ := runIndexDriver(t, mode, pyBase)
		pyLog[mode] = strings.Join(relabelIndex(lines, pyBase), "\n") + "\n"
	}
	pyIndexes := fileTexts(t, filepath.Join(pyBase, "indexes"))
	if *updateS8 {
		_ = os.RemoveAll(golden)
		for mode, text := range pyLog {
			writeFile(t, filepath.Join(golden, mode+".stdout"), []byte(text))
		}
		for rel, text := range pyIndexes {
			writeFile(t, filepath.Join(golden, "indexes", rel), []byte(text))
		}
	}
	for mode, text := range pyLog {
		if want, err := os.ReadFile(filepath.Join(golden, mode+".stdout")); err != nil || string(want) != text {
			t.Errorf("testdata/s8/golden/%s.stdout is not Python's output any more (%v): rerun with -update-s8", mode, err)
		}
	}
	if fmt.Sprint(pyIndexes) != fmt.Sprint(fileTexts(t, filepath.Join(golden, "indexes"))) {
		t.Errorf("testdata/s8/golden/indexes/ is not Python's output any more: rerun with -update-s8")
	}
	for _, b := range []string{pyBase, goBase} {
		s8Corners(t, b)
		s8CleanCorners(t, b)
	}
	for _, mode := range s8Modes {
		indexBoth(t, "corners", mode, pyBase, goBase)
	}
	for _, c := range []struct{ what, keep string }{
		{"empty root", ""},
		{"only bajadoc-clean", "bajadoc-clean"},
		{"only devguide", "devguide"},
		{"only jdk", "jdk"},
	} {
		pyB, goB := filepath.Join(t.TempDir(), "py"), filepath.Join(t.TempDir(), "go")
		for _, b := range []string{pyB, goB} {
			if c.keep == "" {
				writeFile(t, filepath.Join(b, "readme.txt"), nil)
			} else {
				copyTree(t, filepath.Join("testdata", "s8", "root", c.keep), filepath.Join(b, c.keep))
			}
		}
		for _, mode := range s8Modes {
			indexBoth(t, c.what, mode, pyB, goB)
		}
	}
}

// s8Corners adds to a copy of the fixture what a checked-in fixture cannot
// hold for the class index: a symlinked folder (listed, not entered) and a
// symlinked .bajadoc under devguide/, the javadoc list as a symlink.
func s8Corners(t *testing.T, base string) {
	t.Helper()
	dev := filepath.Join(base, "devguide", "zz")
	writeFile(t, filepath.Join(dev, "real", "BLinked.bajadoc"), []byte(`<bajadoc><class qualifiedName="zz.BLinked" name="BLinked" packageName="zz"/></bajadoc>`))
	for link, target := range map[string]string{"alias": "real", "Alias.bajadoc": "real/BLinked.bajadoc"} {
		if err := os.Symlink(target, filepath.Join(dev, link)); err != nil {
			t.Fatal(err)
		}
	}
	list := filepath.Join(base, "bajadoc", "allclasses-noframe.html")
	if err := os.Rename(list, list+".real"); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("allclasses-noframe.html.real", list); err != nil {
		t.Fatal(err)
	}
}

// s8CleanCorners adds bajadoc-clean/ pages that are not UTF-8 (in the
// header, after it in the same decode chunk, past it, a sequence cut by a
// chunk end, a bad continuation after the cut, the blank line's "\r" at a
// chunk end, a truncated end, a header line longer than a chunk) and a
// symlinked folder.
func s8CleanCorners(t *testing.T, base string) {
	t.Helper()
	clean := filepath.Join(base, "bajadoc-clean", "zz")
	head := "CLASS: BBad\nPACKAGE: zz\n\n"         // a whole header
	open := "CLASS: BOpen\nPACKAGE: zz\nNOTE: "    // a header still open
	fill := func(prefix string, room int) string { // prefix padded to room bytes
		return prefix + strings.Repeat("x", room-len(prefix))
	}
	for name, data := range map[string]string{
		"InHeader.txt":    "CLASS: BIn\xffHeader\nPACKAGE: zz\n",
		"AfterHeader.txt": head + "DESCRIPTION:\n\xff\n",              // same decode chunk: still an error
		"PastChunk.txt":   fill(head, textChunk) + "\xff\n",           // next chunk: never read
		"CutAtChunk.txt":  fill(open, textChunk-1) + "\u20ac\xff\n",   // the euro sign spans the chunks
		"CutBad.txt":      fill(open, textChunk-1) + "\xe2\x28\xac\n", // a bad continuation after the cut
		"CrAtChunk.txt":   fill(open, textChunk-2) + "\r\r\xff",       // the blank line's "\r" waits for the next chunk
		"Truncated.txt":   "CLASS: BTrunc\nPACKAGE: zz\nEXTENDS: BObject\n\xe2\x82",
		"LongLine.txt":    "CLASS: BLong\nPACKAGE: zz\nIMPLEMENTS: " + strings.Repeat("BIa, ", 4000) + "BIz\n\n\xff",
	} {
		writeFile(t, filepath.Join(clean, name), []byte(data))
	}
	if err := os.Symlink("../javax", filepath.Join(clean, "linked")); err != nil {
		t.Fatal(err)
	}
}

// A .bajadoc or a bajadoc-clean/ page that cannot be read: class-index and
// method-index stop (Python raises), inheritance lists it under Errors.
func TestParityS8Unreadable(t *testing.T) {
	driver(t)
	if os.Geteuid() == 0 {
		t.Skip("root reads a mode-000 file")
	}
	pyBase, goBase := s8Bases(t)
	for _, b := range []string{pyBase, goBase} {
		for _, p := range []string{filepath.Join(b, "devguide", "baja", "javax", "baja", "sys", "BIObject.bajadoc"),
			filepath.Join(b, "bajadoc-clean", "javax", "baja", "sys", "BMode.txt")} {
			if err := os.Chmod(p, 0); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = os.Chmod(p, 0o644) })
		}
		if err := os.Symlink("missing.bajadoc", filepath.Join(b, "jdk", "java", "Dangling.bajadoc")); err != nil {
			t.Fatal(err)
		}
	}
	for _, mode := range s8Modes {
		pyLines, _ := runIndexDriver(t, mode, pyBase)
		goLines, _ := goIndexRun(mode, goBase)
		diffLines(t, "unreadable "+mode+" lines", relabelIndex(pyLines, pyBase), relabelIndex(goLines, goBase))
	}
}

// s8Versions are the versions TestParityS8Real builds: the
// HELPBUILD_PARITY_INDEXES list when set, else 4.14.0.162 with
// HELPBUILD_PARITY_REAL=1 and 4.15.6.20 with HELPBUILD_PARITY_415=1; none
// is an error, never an empty pass.
func s8Versions() ([]string, error) {
	if env := os.Getenv(s8IndexesVar); env != "" {
		var vids []string
		for _, v := range filepath.SplitList(env) {
			if v != "" {
				vids = append(vids, v)
			}
		}
		if len(vids) == 0 {
			return nil, fmt.Errorf("%s=%q names no version: nothing to compare", s8IndexesVar, env)
		}
		return vids, nil
	}
	var vids []string
	for _, c := range [][2]string{{realParityVar, "4.14.0.162"}, {"HELPBUILD_PARITY_415", "4.15.6.20"}} {
		on, err := parityVar(c[0])
		if err != nil {
			return nil, err
		}
		if on {
			vids = append(vids, c[1])
		}
	}
	if len(vids) == 0 {
		return nil, errors.New("no version to compare: set " + realParityVar + "=1 (4.14.0.162), HELPBUILD_PARITY_415=1 (4.15.6.20) or " + s8IndexesVar)
	}
	return vids, nil
}

// s8Inputs are the folders of a data root the S8 builders read.
var s8Inputs = []string{"bajadoc", "devguide", "guides", "jdk", "bajadoc-clean"}

// realS8Base makes a temporary data root whose S8 input folders link to
// the registered data root of vid (only read; indexes/ is written in the
// temporary root).
func realS8Base(t *testing.T, vid string) string {
	t.Helper()
	return realIndexBase(t, vid, s8Inputs, "bajadoc-clean")
}

// realIndexBase links the inputs of the registered data root of vid into a
// temporary data root; a version without the folder need is a missing
// reference.
func realIndexBase(t *testing.T, vid string, inputs []string, need string) string {
	t.Helper()
	help := niagaraHelpData()
	if help == "" {
		missingReference(t, "no niagara-help data checkout")
	}
	data, err := os.ReadFile(filepath.Join(help, "versions", "registry.json"))
	if err != nil {
		missingReference(t, fmt.Sprintf("no registry: %v", err))
	}
	var reg struct {
		Versions map[string]struct {
			DataRoot string `json:"data_root"`
		} `json:"versions"`
	}
	if err := json.Unmarshal(data, &reg); err != nil {
		t.Fatal(err)
	}
	entry, ok := reg.Versions[vid]
	if !ok || entry.DataRoot == "" {
		t.Fatalf("version %s has no data_root in the registry", vid)
	}
	src := filepath.Join(help, filepath.FromSlash(entry.DataRoot))
	if !pycompat.IsDir(filepath.Join(src, need)) {
		missingReference(t, fmt.Sprintf("version %s has no %s/ (%s)", vid, need, src))
	}
	base := t.TempDir()
	for _, d := range inputs {
		if pycompat.IsDir(filepath.Join(src, d)) {
			if err := os.Symlink(filepath.Join(src, d), filepath.Join(base, d)); err != nil {
				t.Fatal(err)
			}
		}
	}
	return base
}

// TestParityS8Real builds the three indexes of real versions on both sides
// and logs each file's size and both run times.
func TestParityS8Real(t *testing.T) {
	needRealParity(t, "HELPBUILD_PARITY_415", s8IndexesVar)
	driver(t)
	vids, err := s8Versions()
	if err != nil {
		t.Fatal(err)
	}
	for _, vid := range vids {
		t.Run(vid, func(t *testing.T) {
			pyBase, goBase := realS8Base(t, vid), realS8Base(t, vid)
			for _, mode := range s8Modes {
				py, gol := indexBoth(t, vid, mode, pyBase, goBase)
				t.Logf("%s %s: python %.2fs, go %.2fs", vid, mode, py.Seconds(), gol.Seconds())
			}
			for _, name := range []string{"class-index.json", "jdk-class-index.json", "inheritance.json", "method-index.json"} {
				if st, err := os.Stat(filepath.Join(goBase, "indexes", name)); err == nil {
					t.Logf("%s %s: %d bytes identical", vid, name, st.Size())
				}
			}
		})
	}
}

// TestS8Versions: an explicit list is used whole, empty entries dropped; a
// list with no version and no switch at all are errors.
func TestS8Versions(t *testing.T) {
	sep := string(os.PathListSeparator)
	for _, c := range []struct{ list, real, v415, want, err string }{
		{"", "1", "", "4.14.0.162", ""},
		{"", "1", "1", "4.14.0.162 4.15.6.20", ""},
		{"4.3.58.18" + sep + sep, "1", "", "4.3.58.18", ""},
		{sep, "", "", "", "names no version"},
		{"", "", "", "", "no version to compare"},
		{"", "yes", "", "", `HELPBUILD_PARITY_REAL="yes"`},
	} {
		t.Setenv(s8IndexesVar, c.list)
		t.Setenv(realParityVar, c.real)
		t.Setenv("HELPBUILD_PARITY_415", c.v415)
		vids, err := s8Versions()
		if got := strings.Join(vids, " "); got != c.want || (err == nil) != (c.err == "") || err != nil && !strings.Contains(err.Error(), c.err) {
			t.Errorf("%q %q %q: %q, %v; want %q, %q", c.list, c.real, c.v415, got, err, c.want, c.err)
		}
	}
	for _, v := range parityVars {
		t.Setenv(v, "")
	}
	t.Setenv(s8IndexesVar, "4.3.58.18")
	if got := parityRequested(); got != s8IndexesVar+"=4.3.58.18" {
		t.Errorf("%s=4.3.58.18 alone: parityRequested %q", s8IndexesVar, got)
	}
}
