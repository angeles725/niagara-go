package helpbuild

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Parity of the S7a bajadoc parser (bajadoc/javax/baja/**.html ->
// bajadoc-clean/) through testdata/parity/bajadocdriver.py: each side runs
// on its own copy of the data root, then the printed lines (paths shown as
// <base>, the "Done in" seconds as <t>), the exit status and both whole
// trees are compared.

// runBajadocDriver runs bajadocdriver.py and returns its lines and how long
// the interpreter ran.
func runBajadocDriver(t *testing.T, args ...string) ([]string, time.Duration) {
	t.Helper()
	python, tools := driver(t)
	script, err := filepath.Abs(filepath.Join("testdata", "parity", "bajadocdriver.py"))
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(python, append([]string{"-I", "-B", script, tools}, args...)...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	start := time.Now()
	out, err := cmd.Output()
	took := time.Since(start)
	if err != nil {
		t.Fatalf("bajadocdriver %v: %v\n%s", args, err, stderr.String())
	}
	return strings.Split(strings.TrimSuffix(string(out), "\n"), "\n"), took
}

// goBajadocRun is bajadocdriver.py run rebuilt from this package.
func goBajadocRun(base string) ([]string, time.Duration) {
	start := time.Now()
	res, err := ParseBajadoc(base)
	took := time.Since(start)
	var out []string
	if res != nil {
		for _, l := range res.Log {
			out = append(out, dumpLine(str("out"), str(l)))
		}
	}
	if err != nil {
		return append(out, dumpLine(str("raise"))), took
	}
	return append(out, dumpLine(str("exit"), intV(res.Status))), took
}

// relabelBajadoc shows base as <base>, the elapsed seconds as <t> and a
// raise without its class.
func relabelBajadoc(lines []string, base string) []string {
	out := make([]string, len(lines))
	for i, l := range lines {
		if strings.HasPrefix(l, `["raise"`) {
			l = `["raise"]`
		}
		if strings.HasPrefix(l, `["out", "Done in `) {
			l = dumpLine(str("out"), str("Done in <t>s"))
		}
		out[i] = strings.ReplaceAll(l, base, "<base>")
	}
	return out
}

// bajadocBoth runs both sides and compares lines and trees; it returns
// the Python and Go run times. Each of after runs between the runs and the
// tree comparison (unlocking pages the comparison must read).
func bajadocBoth(t *testing.T, what, pyBase, goBase string, after ...func()) (py, gol time.Duration) {
	t.Helper()
	pyLines, py := runBajadocDriver(t, "run", pyBase)
	goLines, gol := goBajadocRun(goBase)
	for _, f := range after {
		f()
	}
	diffLines(t, what+" lines", relabelBajadoc(pyLines, pyBase), relabelBajadoc(goLines, goBase))
	files, size := diffContents(t, pyBase, goBase)
	t.Logf("%s: %d files, %d bytes compared", what, files, size)
	diffLines(t, what+" tree", treeLines(t, pyBase), treeLines(t, goBase))
	return py, gol
}

// bajadocCorners adds what a checked-in fixture cannot hold: an unreadable
// page (unlock makes it readable again), a dangling link, a link to a page
// and one to a folder, a page left under its escaped name and an unrelated
// page bajadoc_xml_parser wrote.
func bajadocCorners(t *testing.T, base string) (unlock func()) {
	t.Helper()
	util := filepath.Join(base, "bajadoc", "javax", "baja", "util")
	locked := filepath.Join(util, "Locked.html")
	writeFile(t, locked, []byte(`<h2 class="title">Class Locked</h2>`))
	if err := os.Chmod(locked, 0); err != nil {
		t.Fatal(err)
	}
	unlock = func() { _ = os.Chmod(locked, 0o644) }
	t.Cleanup(unlock)
	for link, target := range map[string]string{"Dangling.html": "missing.html", "Alias.html": "Cursor.html",
		"linked": "../sys"} {
		if err := os.Symlink(target, filepath.Join(util, link)); err != nil {
			t.Fatal(err)
		}
	}
	clean := filepath.Join(base, "bajadoc-clean", "javax", "baja")
	writeFile(t, filepath.Join(clean, "util", "Cursor&lt;E&gt;.txt"), []byte("old escaped name"))
	writeFile(t, filepath.Join(clean, "xml", "BFromXml.txt"), []byte("CLASS: BFromXml\n"))
	return unlock
}

// TestParityBajadocSynthetic: the checked-in fixture plus the corners,
// twice (the second run overwrites in place), and a data root without
// bajadoc/ (exit 1). It also re-checks the golden files.
func TestParityBajadocSynthetic(t *testing.T) {
	driver(t)
	if os.Geteuid() == 0 {
		t.Skip("root reads a mode-000 page")
	}
	pyBase, goBase := filepath.Join(t.TempDir(), "py"), filepath.Join(t.TempDir(), "go")
	for _, b := range []string{pyBase, goBase} {
		copyTree(t, filepath.Join("testdata", "bajadoc", "root"), b)
	}
	bajadocBoth(t, "fixture", pyBase, goBase)
	golden := fileTexts(t, filepath.Join("testdata", "bajadoc", "clean"))
	if got := fileTexts(t, filepath.Join(pyBase, "bajadoc-clean")); fmt.Sprint(got) != fmt.Sprint(golden) {
		t.Errorf("testdata/bajadoc/clean/ is not Python's output any more: regenerate it (bajadocdriver.py)")
	}
	var unlock []func()
	for _, b := range []string{pyBase, goBase} {
		unlock = append(unlock, bajadocCorners(t, b))
	}
	bajadocBoth(t, "corners", pyBase, goBase, unlock...)
	bajadocBoth(t, "second run (Locked.html readable)", pyBase, goBase)
	empty := []string{filepath.Join(t.TempDir(), "py"), filepath.Join(t.TempDir(), "go")}
	for _, b := range empty {
		writeFile(t, filepath.Join(b, "bajadoc", "javax", "readme.txt"), nil)
	}
	bajadocBoth(t, "no javax/baja", empty[0], empty[1])
}

// bajadocVersions are the versions TestParityBajadocReal builds:
// HELPBUILD_PARITY_BAJADOC (a path-list of version ids) when set, else
// 4.14.0.162 with HELPBUILD_PARITY_REAL=1 and 4.15.6.20 with
// HELPBUILD_PARITY_415=1; none is an error, never an empty pass.
func bajadocVersions() ([]string, error) {
	return versionList("HELPBUILD_PARITY_BAJADOC",
		[][2]string{{realParityVar, "4.14.0.162"}, {"HELPBUILD_PARITY_415", "4.15.6.20"}})
}

// versionList is the version ids named by listVar (a path-list, empty
// entries dropped) when it is set, else those of defaults ({switch, id})
// whose switch is on; none is an error, never an empty pass.
func versionList(listVar string, defaults [][2]string) ([]string, error) {
	if env := os.Getenv(listVar); env != "" {
		var vids []string
		for _, v := range filepath.SplitList(env) {
			if v != "" {
				vids = append(vids, v)
			}
		}
		if len(vids) == 0 {
			return nil, fmt.Errorf("%s=%q names no version: nothing to compare", listVar, env)
		}
		return vids, nil
	}
	var vids []string
	for _, c := range defaults {
		on, err := parityVar(c[0])
		if err != nil {
			return nil, err
		}
		if on {
			vids = append(vids, c[1])
		}
	}
	if len(vids) == 0 {
		return nil, errors.New("no version to compare: set " + realParityVar + "=1, HELPBUILD_PARITY_415=1 or " + listVar)
	}
	return vids, nil
}

// registeredDataRoot is the data root of vid in the niagara-help version
// registry.
func registeredDataRoot(t *testing.T, vid string) string {
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
	return filepath.Join(help, filepath.FromSlash(entry.DataRoot))
}

// realBajadocBase makes a temporary data root whose bajadoc/ links to the
// registered data root of vid (only read; bajadoc-clean/ is written in the
// temporary root).
func realBajadocBase(t *testing.T, vid string) string {
	t.Helper()
	src := filepath.Join(registeredDataRoot(t, vid), "bajadoc")
	if !isDir(filepath.Join(src, "javax", "baja")) {
		missingReference(t, fmt.Sprintf("version %s has no bajadoc/javax/baja (%s)", vid, src))
	}
	base := t.TempDir()
	if err := os.Symlink(src, filepath.Join(base, "bajadoc")); err != nil {
		t.Fatal(err)
	}
	return base
}

// TestParityBajadocReal builds bajadoc-clean/ of real versions on both
// sides and logs both run times.
func TestParityBajadocReal(t *testing.T) {
	needRealParity(t, "HELPBUILD_PARITY_415", "HELPBUILD_PARITY_BAJADOC")
	driver(t)
	vids, err := bajadocVersions()
	if err != nil {
		t.Fatal(err)
	}
	for _, vid := range vids {
		t.Run(vid, func(t *testing.T) {
			pyBase, goBase := realBajadocBase(t, vid), realBajadocBase(t, vid)
			py, gol := bajadocBoth(t, vid, pyBase, goBase)
			t.Logf("%s: python %.1fs, go %.1fs", vid, py.Seconds(), gol.Seconds())
		})
	}
}
