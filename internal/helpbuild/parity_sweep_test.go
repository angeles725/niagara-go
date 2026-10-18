package helpbuild

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/angeles725/niagara-go/internal/help/fts"
	ftbuild "github.com/angeles725/niagara-go/internal/helpbuild/fulltext"
	"github.com/angeles725/niagara-go/internal/index"
)

// Parity of the whole-install sweep (install_sweep.cmd_sweep) through
// testdata/parity/sweepdriver.py: each side runs the gate with a
// repository of its own (a copy of the registry, so each writes its own
// jar-listing cache) over the same install and data root; the report
// lines, the exit status and both caches are compared byte for byte.

// sweepParityVar selects the real versions of TestParitySweepReal (a
// path-list of version ids); it is one of parityVars.
const sweepParityVar = "HELPBUILD_PARITY_SWEEP"

// runSweepDriver runs sweepdriver.py and returns its lines (the "took"
// line removed) and the gate's own run time.
func runSweepDriver(t *testing.T, args ...string) ([]string, time.Duration) {
	t.Helper()
	python, tools := driver(t)
	script, err := filepath.Abs(filepath.Join("testdata", "parity", "sweepdriver.py"))
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(python, append([]string{"-I", "-B", script, tools}, args...)...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("sweepdriver %v: %v\n%s", args, err, stderr.String())
	}
	lines := strings.Split(strings.TrimSuffix(string(out), "\n"), "\n")
	var took time.Duration
	if n := len(lines); n > 0 && strings.HasPrefix(lines[n-1], `["took", `) {
		secs, _ := strconv.ParseFloat(strings.TrimSuffix(strings.TrimPrefix(lines[n-1], `["took", `), "]"), 64)
		took, lines = time.Duration(secs*float64(time.Second)), lines[:n-1]
	}
	return lines, took
}

// sweepLines is the driver's stream: each printed report line as
// ["out", line], then ["exit", status].
func sweepLines(report string, status int) []string {
	var out []string
	if report != "" {
		for _, line := range strings.Split(strings.TrimSuffix(report, "\n"), "\n") {
			out = append(out, dumpLine(str("out"), str(line)))
		}
	}
	return append(out, fmt.Sprintf(`["exit", %d]`, status))
}

// goSweep runs RunSweep like the driver's sweep mode.
func goSweep(repo, base, vid, rulesPath string) ([]string, time.Duration, error) {
	var opts SweepOptions
	if rulesPath != "-" {
		r, err := Load(rulesPath)
		if err != nil {
			return nil, 0, err
		}
		opts.Rules = r
	}
	var buf bytes.Buffer
	start := time.Now()
	status, err := RunSweep(repo, base, vid, &buf, opts)
	took := time.Since(start)
	if err != nil {
		return nil, took, err
	}
	return sweepLines(buf.String(), status), took, nil
}

// sweepRepos makes the two repositories of one comparison: each holds a
// copy of the registry at src (and, when seed is not "", the jar-listing
// cache file seed under the cache path of install).
func sweepRepos(t *testing.T, src, install, seed string) (py, gol string) {
	t.Helper()
	reg, err := os.ReadFile(filepath.Join(src, index.RegistryRel))
	if err != nil {
		t.Fatal(err)
	}
	var repos [2]string
	for i := range repos {
		repos[i] = t.TempDir()
		writeFile(t, filepath.Join(repos[i], index.RegistryRel), reg)
		if seed != "" {
			cache, err := CachePath(repos[i], install)
			if err != nil {
				t.Fatal(err)
			}
			copyFile(t, seed, cache)
		}
	}
	return repos[0], repos[1]
}

// compareSweep runs the gate on both sides and compares the streams and
// the caches each side wrote; result is Go's last report line.
func compareSweep(t *testing.T, what, pyRepo, goRepo, install, base, vid, rules string) (py, gol time.Duration, result string) {
	t.Helper()
	want, py := runSweepDriver(t, "sweep", pyRepo, base, vid, rules)
	got, gol, err := goSweep(goRepo, base, vid, rules)
	if err != nil {
		t.Fatalf("%s: Go raised %v; Python printed\n%s", what, err, strings.Join(want, "\n"))
	}
	diffLines(t, what+" report", want, got)
	if n := len(got); n > 1 {
		result = got[n-2]
	}
	pc, _ := CachePath(pyRepo, install)
	gc, _ := CachePath(goRepo, install)
	pb, perr := os.ReadFile(pc)
	gb, gerr := os.ReadFile(gc)
	if (perr == nil) != (gerr == nil) || !bytes.Equal(pb, gb) {
		t.Errorf("%s: jar-listing caches differ: Python %d bytes (%v), Go %d bytes (%v)", what, len(pb), perr, len(gb), gerr)
	}
	return py, gol, result
}

// lockDir makes dir unreadable for the test and reports whether that
// worked (root reads mode-0 folders).
func lockDir(t *testing.T, dir string) bool {
	t.Helper()
	if err := os.Chmod(dir, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })
	_, err := os.ReadDir(dir)
	return err != nil
}

// TestParitySweepSynthetic compares the gate on the driver's fixture:
// every content kind, the installed-copy and in-jar-duplicate rules, empty,
// corrupt, upper-case and non-ASCII jars, a root jar, a location longer
// than its column, more than 25 gaps, loose docs (PDFs in and out of
// pdf-index.json, any case), unreadable folders in the install and under
// docs/, the course row, a version sharing the install but never
// extracted (PDF fallback, course note), an unavailable install, an
// unknown version and an identified one; with the shipped rules and the
// test rules; then again over each side's warm cache.
func TestParitySweepSynthetic(t *testing.T) {
	driver(t)
	dir := t.TempDir()
	if out, _ := runSweepDriver(t, "fixture", dir); len(out) != 1 || out[0] != `["ok"]` {
		t.Fatalf("fixture: %q", out)
	}
	install := filepath.Join(dir, "install")
	locked := lockDir(t, filepath.Join(install, "sw", "locked"))
	locked = lockDir(t, filepath.Join(install, "docs", "locked")) && locked
	if !locked {
		t.Log("running with permissions that read mode-0 folders: the unreadable-folder rows are not exercised")
	}
	repo := filepath.Join(dir, "repo")
	rules := filepath.Join(dir, "rules.json")
	for _, c := range []struct{ name, vid, base, rules string }{
		{"1.0 shipped rules", "1.0", "1.0", "-"},
		{"1.0 test rules", "1.0", "1.0", rules},
		{"identified from the data root", "", "1.0", "-"},
		{"2.0 nothing extracted", "2.0", "2.0", "-"},
		{"3.0 no install", "3.0", "3.0", "-"},
		{"unknown version", "9.9", "1.0", "-"},
		{"unregistered data root", "", "none", "-"},
	} {
		t.Run(c.name, func(t *testing.T) {
			base := filepath.Join(repo, "versions", c.base)
			pyRepo, goRepo := sweepRepos(t, repo, install, "")
			compareSweep(t, "cold", pyRepo, goRepo, install, base, c.vid, c.rules)
			compareSweep(t, "warm", pyRepo, goRepo, install, base, c.vid, c.rules)
		})
	}
}

// sweepVersions are the versions of TestParitySweepReal.
func sweepVersions() ([]string, error) {
	env := os.Getenv(sweepParityVar)
	var vids []string
	for _, v := range filepath.SplitList(env) {
		if v != "" {
			vids = append(vids, v)
		}
	}
	if len(vids) == 0 {
		return nil, fmt.Errorf("%s=%q names no version: nothing to compare", sweepParityVar, env)
	}
	return vids, nil
}

// freshFulltextOutside makes sure both sides find an up-to-date full-text
// index of base without writing into it: Python's open_index would rebuild
// a stale indexes/fulltext.sqlite inside niagara-help. When the data root's
// own index is stale, the test points XDG_CACHE_HOME (open_index's second
// place, read by both sides) at a temporary folder and builds the index
// there with the Go builder.
func freshFulltextOutside(t *testing.T, base string) {
	t.Helper()
	if _, err := FreshFulltextPath(base); err == nil {
		return
	}
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	start := time.Now()
	res, err := ftbuild.Build(base, ftbuild.Options{Output: fts.CachePath(base)})
	if err != nil {
		t.Fatalf("building the full-text index of %s in a temporary cache: %v", base, err)
	}
	t.Logf("%s: its indexes/fulltext.sqlite is stale; built %d documents in a temporary XDG_CACHE_HOME (%.1f s)",
		base, res.Documents, time.Since(start).Seconds())
	if _, err := FreshFulltextPath(base); err != nil {
		t.Fatal(err)
	}
}

// TestParitySweepReal runs the whole gate on real registered versions:
// each side gets its own repository (a registry copy seeded with
// niagara-help's jar-listing cache of the install, when there is one), so
// nothing is written into niagara-help or the install (see
// freshFulltextOutside for the full-text index).
func TestParitySweepReal(t *testing.T) {
	if os.Getenv(sweepParityVar) == "" {
		t.Skipf("real-data parity runs only on request: set %s=<version id>[:...]", sweepParityVar)
	}
	vids, err := sweepVersions()
	if err != nil {
		t.Fatal(err)
	}
	driver(t)
	help := filepath.Dir(niagaraHelpTools())
	data, err := os.ReadFile(filepath.Join(help, index.RegistryRel))
	if err != nil {
		t.Fatal(err)
	}
	var reg struct {
		Versions map[string]struct {
			InstallDir string `json:"install_dir"`
			DataRoot   string `json:"data_root"`
		} `json:"versions"`
	}
	if err := json.Unmarshal(data, &reg); err != nil {
		t.Fatal(err)
	}
	for _, vid := range vids {
		t.Run(vid, func(t *testing.T) {
			entry, ok := reg.Versions[vid]
			if !ok {
				t.Fatalf("version %s is not registered in %s", vid, help)
			}
			if !isDir(entry.InstallDir) {
				t.Fatalf("install of %s not mounted: %s", vid, entry.InstallDir)
			}
			base := filepath.Join(help, filepath.FromSlash(entry.DataRoot))
			freshFulltextOutside(t, base)
			seed, _ := CachePath(help, entry.InstallDir)
			if _, err := os.Stat(seed); err != nil {
				seed = ""
			}
			pyRepo, goRepo := sweepRepos(t, help, entry.InstallDir, seed)
			py, gol, result := compareSweep(t, vid, pyRepo, goRepo, entry.InstallDir, base, vid, "-")
			t.Logf("%s: %s; Python %.2f s, Go %.2f s (cache seeded: %v)", vid, result, py.Seconds(), gol.Seconds(), seed != "")
		})
	}
}

// An explicit HELPBUILD_PARITY_SWEEP is used whole, empty entries
// dropped; a list with no version is an error.
func TestSweepVersions(t *testing.T) {
	sep := string(os.PathListSeparator)
	t.Setenv(sweepParityVar, "4.3.58.18"+sep+sep+"4.15.6.20")
	if vids, err := sweepVersions(); err != nil || strings.Join(vids, " ") != "4.3.58.18 4.15.6.20" {
		t.Errorf("list: %q %v", vids, err)
	}
	t.Setenv(sweepParityVar, sep)
	if _, err := sweepVersions(); err == nil || !strings.Contains(err.Error(), sweepParityVar) {
		t.Errorf("empty list: %v", err)
	}
}
