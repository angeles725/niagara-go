package helpbuild

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Real-data parity of the coverage reports: `coverage`, `coverage
// alarm-rt`, the guide coverage and `coverage --doc-jars` on a registered
// version's data root and install, Python against Go, plus (for the
// version the niagara-help CLI goldens record) the SHA-256 of each Go
// report against tests/golden/cli.json.

// covVar selects the versions of TestParityCoverageReal (a path-list of
// version ids); it is one of parityVars, so it counts as a parity request.
const covVar = "HELPBUILD_PARITY_COVERAGE"

// covGoldenVersion is golden_cli.VERSION, the version cli.json records.
const covGoldenVersion = "4.14.0.162"

// covVersions are the versions to compare: the covVar list, else
// 4.14.0.162 with HELPBUILD_PARITY_REAL=1 and 4.15.6.20 with
// HELPBUILD_PARITY_415=1; explicit is true for a covVar list.
func covVersions() (vids []string, explicit bool, err error) {
	if env := os.Getenv(covVar); env != "" {
		for _, v := range filepath.SplitList(env) {
			if v != "" {
				vids = append(vids, v)
			}
		}
		if len(vids) == 0 {
			return nil, true, fmt.Errorf("%s=%q names no version: nothing to compare", covVar, env)
		}
		return vids, true, nil
	}
	for _, c := range [][2]string{{realParityVar, covGoldenVersion}, {"HELPBUILD_PARITY_415", "4.15.6.20"}} {
		on, err := parityVar(c[0])
		if err != nil {
			return nil, false, err
		}
		if on {
			vids = append(vids, c[1])
		}
	}
	if len(vids) == 0 {
		return nil, false, errors.New("no version to compare: set " + realParityVar + "=1 (" + covGoldenVersion +
			"), HELPBUILD_PARITY_415=1 (4.15.6.20) or " + covVar)
	}
	return vids, false, nil
}

// TestCovVersions: the selection never leaves nothing to compare.
func TestCovVersions(t *testing.T) {
	sep := string(os.PathListSeparator)
	for _, c := range []struct {
		cov, real, v415 string
		want            string
		explicit        bool
		err             string
	}{
		{"", "1", "", covGoldenVersion, false, ""},
		{"", "1", "1", covGoldenVersion + " 4.15.6.20", false, ""},
		{"4.3.58.18" + sep + sep + "4.13.2.18", "", "", "4.3.58.18 4.13.2.18", true, ""},
		{sep, "1", "", "", true, "names no version"},
		{"", "", "", "", false, "no version to compare"},
		{"", "yes", "", "", false, realParityVar + `="yes"`},
	} {
		t.Setenv(covVar, c.cov)
		t.Setenv(realParityVar, c.real)
		t.Setenv("HELPBUILD_PARITY_415", c.v415)
		vids, explicit, err := covVersions()
		if got := strings.Join(vids, " "); got != c.want || explicit != c.explicit ||
			(err == nil) != (c.err == "") || err != nil && !strings.Contains(err.Error(), c.err) {
			t.Errorf("%q %q %q: %q %v %v; want %q %v %q", c.cov, c.real, c.v415, got, explicit, err, c.want, c.explicit, c.err)
		}
	}
}

// covRegistryEntry is a version's data root (absolute) and install.
func covRegistryEntry(t *testing.T, help, vid string) (root, install string, registry []byte) {
	t.Helper()
	registry, err := os.ReadFile(filepath.Join(help, "versions", "registry.json"))
	if err != nil {
		missingReference(t, fmt.Sprintf("no registry: %v", err))
	}
	var reg struct {
		Versions map[string]struct {
			DataRoot   string `json:"data_root"`
			InstallDir string `json:"install_dir"`
		} `json:"versions"`
	}
	if err := json.Unmarshal(registry, &reg); err != nil {
		t.Fatal(err)
	}
	e, ok := reg.Versions[vid]
	if !ok || e.DataRoot == "" {
		t.Fatalf("version %s has no data_root in the registry", vid)
	}
	return filepath.Clean(filepath.Join(help, filepath.FromSlash(e.DataRoot))), e.InstallDir, registry
}

// covGolden is one case of niagara-help tests/golden/cli.json.
type covGolden struct {
	RC     int    `json:"rc"`
	Stdout string `json:"stdout"`
}

func covGoldens(t *testing.T, help string) map[string]covGolden {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(help, "tests", "golden", "cli.json"))
	if err != nil {
		t.Fatalf("CLI goldens: %v", err)
	}
	var all map[string]json.RawMessage
	if err := json.Unmarshal(data, &all); err != nil {
		t.Fatal(err)
	}
	out := map[string]covGolden{}
	for _, id := range []string{"coverage", "coverage-module", "coverage-doc-jars"} {
		var g covGolden
		if err := json.Unmarshal(all[id], &g); err != nil {
			t.Fatalf("golden %s: %v", id, err)
		}
		out[id] = g
	}
	return out
}

// checkCovGolden compares a Go report with a CLI golden case.
func checkCovGolden(t *testing.T, goldens map[string]covGolden, id, out string, rc int) {
	t.Helper()
	sum := sha256.Sum256([]byte(out))
	if g := goldens[id]; hex.EncodeToString(sum[:]) != g.Stdout || rc != g.RC {
		t.Errorf("golden %s: stdout sha256 %x rc %d; cli.json records %s rc %d", id, sum, rc, g.Stdout, g.RC)
	} else {
		t.Logf("golden %s: stdout sha256 and rc %d match cli.json (%d bytes)", id, rc, len(out))
	}
}

// timedCovBoth is covBoth on one shared root, logging both run times.
func timedCovBoth(t *testing.T, what, mode string, pyArgs []string, goRun func() []string) {
	t.Helper()
	start := time.Now()
	py := runCovDriver(t, append([]string{mode}, pyArgs...)...)
	pyTook := time.Since(start)
	if len(py) != 2 {
		t.Fatalf("%s: the driver printed %d lines, want 2: %q", what, len(py), py)
	}
	start = time.Now()
	gol := goRun()
	goTook := time.Since(start)
	diffLines(t, what, py, gol)
	t.Logf("%s: %d bytes identical; python %.2fs, go %.2fs (driver start-up included on the Python side)",
		what, len(py[0]), pyTook.Seconds(), goTook.Seconds())
}

// TestParityCoverageReal: the reports on real versions, read from the live
// niagara-help checkout (registry, data roots), the Python side and the CLI
// goldens taken from the pre-go-retire reference. The data root is
// only read (its full-text index must be up to date, else Python would
// rebuild it in place); each side saves its listing cache in a repository
// of its own holding a copy of the registry.
func TestParityCoverageReal(t *testing.T) {
	needRealParity(t, "HELPBUILD_PARITY_415", covVar)
	_, refTools := covPython(t)
	vids, explicit, err := covVersions()
	if err != nil {
		t.Fatal(err)
	}
	live := niagaraHelpTools()
	if live == "" {
		missingReference(t, "no niagara-help checkout (the registry and data roots): parity not checked")
	}
	help := filepath.Dir(live)
	for _, vid := range vids {
		t.Run(vid, func(t *testing.T) {
			root, install, registry := covRegistryEntry(t, help, vid)
			for _, mode := range [][]string{{"coverage"}, {"coverage", "alarm-rt"}, {"guides"}} {
				args := append([]string{root}, mode[1:]...)
				timedCovBoth(t, vid+" "+strings.Join(mode, " "), mode[0], args, func() []string { return goCov(mode[0], args...) })
			}
			var goldens map[string]covGolden
			if vid == covGoldenVersion { // the two reports that need no install
				goldens = covGoldens(t, filepath.Dir(refTools))
				var b strings.Builder
				res, err := Coverage(&b, root, "")
				if err != nil {
					t.Fatal(err)
				}
				checkCovGolden(t, goldens, "coverage", b.String(), min(res.GuideGaps, 1))
				b.Reset()
				if _, err := Coverage(&b, root, "alarm-rt"); err != nil {
					t.Fatal(err)
				}
				checkCovGolden(t, goldens, "coverage-module", b.String(), 0)
			}
			if !isDir(install) {
				if explicit {
					t.Fatalf("%s: install %s not mounted (%s lists the version)", vid, install, covVar)
				}
				t.Skipf("%s: install %s not mounted: coverage --doc-jars not compared", vid, install)
			}
			if _, err := freshDocs(root); err != nil {
				t.Fatalf("%s: %v; rebuild it first (nh rebuild-all), the parity run never writes the corpus", vid, err)
			}
			repos := [2]string{filepath.Join(t.TempDir(), "py"), filepath.Join(t.TempDir(), "go")}
			for _, r := range repos {
				writeFile(t, filepath.Join(r, "versions", "registry.json"), registry)
			}
			timedCovBoth(t, vid+" --doc-jars", "docjars", []string{repos[0], root, vid},
				func() []string { return goCovDocJars(freshDocs, repos[1], root, vid) })
			pyCache, goCache := fileTexts(t, filepath.Join(repos[0], ".cache")), fileTexts(t, filepath.Join(repos[1], ".cache"))
			if len(pyCache) == 0 {
				t.Fatalf("%s: Python saved no listing cache under %s: nothing to compare", vid, repos[0])
			}
			for rel, text := range pyCache {
				if goCache[rel] != text {
					t.Errorf("%s: listing cache %s differs", vid, rel)
				}
			}
			if len(goCache) != len(pyCache) {
				t.Errorf("%s: listing caches: python %d files, go %d", vid, len(pyCache), len(goCache))
			}
			if vid != covGoldenVersion {
				return
			}
			var b strings.Builder
			rc, err := DocJarCoverage(&b, repos[1], root, vid, freshDocs)
			if err != nil {
				t.Fatal(err)
			}
			checkCovGolden(t, goldens, "coverage-doc-jars", b.String(), rc)
		})
	}
}
