package navbuild

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"
)

// The Python-vs-Go parity harness: each test runs a real module-navigator
// builder (python3 tools/build_*.py --base-dir <temp>) and the Go builder into
// another temp base dir over the same organized/ tree, then byte-diffs the two
// indexes after NormalizeVolatile. It needs python3 and a module-navigator
// checkout whose builders take --base-dir (fix/deterministic-xref-base-dir):
// $NAVBUILD_PY_NAV, else $NIAGARA_RESEARCH/module-navigator (default
// /home/cristian/niagara-research/module-navigator). It skips visibly without.
//
// Corpora: testdata/organized (synthetic, always); a few real modules of the
// organized/ corpus linked into a temp tree ($NAVBUILD_PARITY_MODULES, comma
// separated, skipped with -short); the whole corpus with NAVBUILD_PARITY_FULL=1.

// update rewrites testdata/golden/<index>.golden from the PYTHON build of the
// synthetic corpus (normalised), so the Python-free golden tests keep checking
// what the parity tests proved.
var update = flag.Bool("update", false, "rewrite testdata/golden from the Python builders")

// defaultRealModules cover a ZKM submodule (alarm-rt), a module without recon
// output and with third-party packages (ColdRoomPan), one without code
// (CentralineAhuPx), and two ordinary ones.
const defaultRealModules = "alarm,backup,ColdRoomPan,CentralineAhuPx,kitControl"

func researchRoot() string {
	if root := os.Getenv("NIAGARA_RESEARCH"); root != "" {
		return root
	}
	return "/home/cristian/niagara-research"
}

func pythonNav(t *testing.T) (python, nav string) {
	t.Helper()
	python, err := exec.LookPath("python3")
	if err != nil {
		t.Skip("python3 not on PATH: parity not checked")
	}
	nav = os.Getenv("NAVBUILD_PY_NAV")
	if nav == "" {
		nav = filepath.Join(researchRoot(), "module-navigator")
	}
	src, err := os.ReadFile(filepath.Join(nav, "tools", "build_module_inventory.py"))
	if err != nil {
		t.Skipf("module-navigator builders not available (%v): parity not checked", err)
	}
	if !bytes.Contains(src, []byte("--base-dir")) {
		t.Skipf("%s predates the builders' --base-dir (module-navigator fix/deterministic-xref-base-dir): parity not checked", nav)
	}
	return python, nav
}

// runPython runs tools/<script> --base-dir base [args] over organized, logs
// its wall time and peak RSS and returns the wall time. -E drops
// PYTHONHASHSEED, so set order is not pinned either.
func runPython(t *testing.T, script, base, organized string, args ...string) time.Duration {
	t.Helper()
	python, nav := pythonNav(t)
	cmd := measured(python, append([]string{"-E", "-B", script, "--base-dir", base}, args...)...)
	cmd.Dir = filepath.Join(nav, "tools")
	cmd.Env = append(os.Environ(), "NAV_CORPUS_BASE="+organized)
	t0 := time.Now()
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s: %v\n%s", script, err, out)
	}
	d := time.Since(t0)
	t.Logf("python %s: %v, peak RSS %s", script, d.Round(time.Millisecond), peakRSS(out))
	return d
}

// gnuTime is GNU time, which reports a child's peak RSS: the rusage of a
// child of this (large) test process would report this process's peak
// instead, since Linux keeps the high-water mark across exec.
const gnuTime = "/usr/bin/time"

// measured is exec.Command(name, args...), run under GNU time when it exists.
func measured(name string, args ...string) *exec.Cmd {
	if isFile(gnuTime) {
		return exec.Command(gnuTime, append([]string{"-f", "NAVBUILD_PEAK_RSS_KB=%M", name}, args...)...)
	}
	return exec.Command(name, args...)
}

var peakRSSLine = regexp.MustCompile(`NAVBUILD_PEAK_RSS_KB=([0-9]+)`)

// peakRSS is the peak RSS GNU time reported in out.
func peakRSS(out []byte) string {
	m := peakRSSLine.FindSubmatch(out)
	if m == nil {
		return "n/a"
	}
	kb, _ := strconv.Atoi(string(m[1]))
	return fmt.Sprintf("%d MiB", kb/1024)
}

// newBase is an empty module-navigator base dir (with indexes/, as the
// builders require of an explicit --base-dir).
func newBase(t *testing.T) string {
	t.Helper()
	base := t.TempDir()
	if err := os.Mkdir(filepath.Join(base, "indexes"), 0o755); err != nil {
		t.Fatal(err)
	}
	return base
}

// sameIndex fails unless the two indexes are equal after NormalizeVolatile,
// showing the first differing line.
func sameIndex(t *testing.T, name, pyBase, goBase string) {
	t.Helper()
	if name == TokenFile {
		sameTokenDB(t, IndexPath(pyBase, name), IndexPath(goBase, name))
		return
	}
	py, err := os.ReadFile(IndexPath(pyBase, name))
	if err != nil {
		t.Fatal(err)
	}
	gob, err := os.ReadFile(IndexPath(goBase, name))
	if err != nil {
		t.Fatal(err)
	}
	py, gob = NormalizeVolatile(py), NormalizeVolatile(gob)
	if *update && filepath.Base(t.Name()) == "synthetic" {
		if err := os.WriteFile(goldenPath(name), py, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if bytes.Equal(py, gob) {
		t.Logf("%s: %d bytes identical after normalisation", name, len(py))
		return
	}
	t.Errorf("%s differs (python %d bytes, go %d bytes): %s", name, len(py), len(gob), firstDiff(py, gob))
}

func firstDiff(a, b []byte) string {
	al, bl := strings.Split(string(a), "\n"), strings.Split(string(b), "\n")
	for i := 0; i < len(al) || i < len(bl); i++ {
		var x, y string
		if i < len(al) {
			x = al[i]
		}
		if i < len(bl) {
			y = bl[i]
		}
		if x != y {
			return fmt.Sprintf("line %d\n  python: %.300q\n  go:     %.300q", i+1, x, y)
		}
	}
	return "only the line structure differs"
}

// syntheticCorpus is testdata/organized, absolute.
func syntheticCorpus(t *testing.T) string {
	t.Helper()
	p, err := filepath.Abs(filepath.Join("testdata", "organized"))
	if err != nil {
		t.Fatal(err)
	}
	return p
}

// realOrganized is the organized/ corpus ($NAV_CORPUS_BASE, else
// $NIAGARA_RESEARCH/organized); the test skips without it.
func realOrganized(t *testing.T) string {
	t.Helper()
	pythonNav(t)
	org := os.Getenv("NAV_CORPUS_BASE")
	if org == "" {
		org = filepath.Join(researchRoot(), "organized")
	}
	if !isDir(org) {
		t.Skipf("organized/ corpus not found at %s", org)
	}
	return org
}

// realSubset is a temp organized/ holding symlinks to a few real modules.
func realSubset(t *testing.T) string {
	t.Helper()
	if testing.Short() {
		t.Skip("real-corpus parity skipped with -short")
	}
	org := realOrganized(t)
	mods := os.Getenv("NAVBUILD_PARITY_MODULES")
	if mods == "" {
		mods = defaultRealModules
	}
	subset := t.TempDir()
	for _, m := range strings.Split(mods, ",") {
		src := filepath.Join(org, m)
		if !isDir(src) {
			t.Fatalf("module %s not in %s", m, org)
		}
		if err := os.Symlink(src, filepath.Join(subset, m)); err != nil {
			t.Fatal(err)
		}
	}
	return subset
}

// fullCorpus is the whole organized/ corpus, only with NAVBUILD_PARITY_FULL=1.
func fullCorpus(t *testing.T) string {
	t.Helper()
	if os.Getenv("NAVBUILD_PARITY_FULL") != "1" {
		t.Skip("full-corpus parity runs only with NAVBUILD_PARITY_FULL=1")
	}
	return realOrganized(t)
}

func TestParityInventory(t *testing.T) {
	for _, tc := range []struct {
		name   string
		corpus func(*testing.T) string
	}{
		{"synthetic", syntheticCorpus},
		{"real-modules", realSubset},
		{"full-corpus", fullCorpus},
	} {
		t.Run(tc.name, func(t *testing.T) {
			organized := tc.corpus(t)
			pyBase, goBase := newBase(t), newBase(t)
			pyTime := runPython(t, "build_module_inventory.py", pyBase, organized)
			t0 := time.Now()
			if _, err := BuildInventory(goBase, organized); err != nil {
				t.Fatal(err)
			}
			t.Logf("module-inventory: python %v, go %v", pyTime.Round(time.Millisecond), time.Since(t0).Round(time.Millisecond))
			sameIndex(t, InventoryFile, pyBase, goBase)
		})
	}
}
