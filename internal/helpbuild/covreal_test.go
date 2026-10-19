package helpbuild

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/angeles725/niagara-go/internal/help/fts"
	"github.com/angeles725/niagara-go/internal/pycompat"
)

// The coverage reports on the real corpus of the version niagara-help's
// CLI goldens record: the rc and the SHA-256 of each Go report against
// those goldens, copied from niagara-help tests/golden/cli.json at
// pre-go-retire into testdata/cov/cli-goldens.json.

// covGoldenVersion is golden_cli.VERSION, the version cli.json records.
const covGoldenVersion = "4.14.0.162"

// covGoldensFile holds the coverage cases of niagara-help
// tests/golden/cli.json as captured at the tag pre-go-retire (8410d7451).
var covGoldensFile = filepath.Join("testdata", "cov", "cli-goldens.json")

// covRegistryEntry is a version's data root (absolute) and install.
func covRegistryEntry(t *testing.T, help, vid string) (root, install string, registry []byte) {
	t.Helper()
	registry, err := os.ReadFile(filepath.Join(help, "versions", "registry.json"))
	if err != nil {
		t.Fatalf("no registry: %v", err)
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

// covGoldens reads the copied cases (covGoldensFile); a case missing from
// it is an error, never a zero hash.
func covGoldens(t *testing.T) map[string]covGolden {
	t.Helper()
	data, err := os.ReadFile(covGoldensFile)
	if err != nil {
		t.Fatalf("CLI goldens: %v", err)
	}
	out, err := parseCovGoldens(data, covGoldenIDs)
	if err != nil {
		t.Fatalf("CLI goldens %s: %v", covGoldensFile, err)
	}
	return out
}

// covGoldenIDs are the cases checkCovGolden compares.
var covGoldenIDs = []string{"coverage", "coverage-module", "coverage-doc-jars"}

// parseCovGoldens decodes the cases ids of a cli.json; a case that is
// missing or not a JSON object (null included) is an error.
func parseCovGoldens(data []byte, ids []string) (map[string]covGolden, error) {
	var all map[string]json.RawMessage
	if err := json.Unmarshal(data, &all); err != nil {
		return nil, err
	}
	out := map[string]covGolden{}
	for _, id := range ids {
		var g covGolden
		raw, has := all[id]
		if !has {
			return nil, fmt.Errorf("golden %s: not in the file", id)
		}
		// json.Unmarshal of null into a struct is a no-op: the zero case.
		if raw = bytes.TrimSpace(raw); len(raw) == 0 || raw[0] != '{' {
			return nil, fmt.Errorf("golden %s: %s, not an object", id, raw)
		}
		if err := json.Unmarshal(raw, &g); err != nil {
			return nil, fmt.Errorf("golden %s: %v", id, err)
		}
		out[id] = g
	}
	return out, nil
}

// A case missing, stored as JSON null or not an object is an error: each
// would otherwise compare as rc 0 with an empty stdout.
func TestParseCovGoldensRejectsAMissingCase(t *testing.T) {
	ok := `{"rc": 0, "stdout": "x"}`
	if got, err := parseCovGoldens([]byte(`{"a": `+ok+`}`), []string{"a"}); err != nil || got["a"].Stdout != "x" {
		t.Fatalf("a good case: %+v, %v", got, err)
	}
	for name, text := range map[string]string{
		"missing":   `{"b": ` + ok + `}`,
		"null":      `{"a": null}`,
		"spaced":    `{"a":  null }`,
		"string":    `{"a": "x"}`,
		"no object": `[]`,
	} {
		if got, err := parseCovGoldens([]byte(text), []string{"a"}); err == nil {
			t.Errorf("%s: accepted as %+v", name, got)
		}
	}
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

// freshDocs is FulltextDocs that never builds: a real corpus must already
// hold an up-to-date index (the test never writes the corpus).
func freshDocs(root string) (map[string]bool, error) {
	path, _ := fts.FreshPath(root, fts.Fingerprint(fts.SourceDirs(root)))
	if path == "" {
		return nil, errors.New("the full-text index of " + root + " is missing or stale")
	}
	return FulltextDocsAt(path)
}

// TestCoverageRealGoldens checks the Go reports of covGoldenVersion
// against covGoldensFile: coverage, coverage alarm-rt and, when the
// install is mounted, coverage --doc-jars. It reads the niagara-help
// checkout named by NIAGARA_HELP_DIR (registry, data root) as data only, so
// the data root's full-text index must be up to date; the listing cache
// goes to a temporary repository holding a copy of the registry. It skips
// when the variable is unset or with -short; make realdata sets it.
func TestCoverageRealGoldens(t *testing.T) {
	help := os.Getenv("NIAGARA_HELP_DIR")
	if help == "" {
		t.Skip("NIAGARA_HELP_DIR is not set: the real coverage goldens need the niagara-help checkout")
	}
	if testing.Short() {
		t.Skip("the real coverage goldens read the real corpus and install")
	}
	goldens := covGoldens(t)
	root, install, registry := covRegistryEntry(t, help, covGoldenVersion)
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
	if !pycompat.IsDir(install) {
		t.Skipf("install %s not mounted: coverage --doc-jars not checked", install)
	}
	if _, err := freshDocs(root); err != nil {
		t.Fatalf("%v; rebuild it first (nh rebuild-all), this test never writes the corpus", err)
	}
	repo := t.TempDir()
	writeFile(t, filepath.Join(repo, "versions", "registry.json"), registry)
	b.Reset()
	rc, err := DocJarCoverage(&b, repo, root, covGoldenVersion, freshDocs)
	if err != nil {
		t.Fatal(err)
	}
	checkCovGolden(t, goldens, "coverage-doc-jars", b.String(), rc)
}
