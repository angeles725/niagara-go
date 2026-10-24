package helpbuild

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/angeles725/niagara-go/internal/golden"
	"github.com/angeles725/niagara-go/internal/help/fts"
	"github.com/angeles725/niagara-go/internal/helpbuild/internal/testkit"
	"github.com/angeles725/niagara-go/internal/pycompat"
)

// The coverage reports on the real corpus of covGoldenVersion are recorded from Go under
// testdata/cov/real/: one <case>.txt per report and recorded.json with each case's exit status,
// the corpus stamp they were read from and a digest of the report code. Two checks keep them from
// going stale silently (the Python-era hashes did after 8bdcc06c changed the report wording):
// TestCoverageRealRecordingMatchesCode runs in CI and fails when the report code changed since the
// recording; TestCoverageRealGoldens (make realdata) fails first when the corpus was re-stamped since.
// Both name the fix: re-record with -update-cov-real and review the diff.

var updateCovReal = flag.Bool("update-cov-real", false,
	"rewrite testdata/cov/real/ from the Go reports on the real corpus (needs NIAGARA_HELP_DIR; review the diff)")

// covGoldenVersion is the version whose real corpus the reports are recorded on.
const covGoldenVersion = "4.14.0.162"

var covRealDir = filepath.Join("testdata", "cov", "real")

// covCodeFiles are the files whose code writes the coverage reports.
var covCodeFiles = []string{
	"coveragereport.go", "coveragereportmodule.go", "doccoverage.go", "doccoverageclassify.go", "doccoveragereport.go",
}

const covReRecord = "re-record coverage goldens: NIAGARA_HELP_DIR=<niagara-help> go test ./internal/helpbuild " +
	"-run '^TestCoverageRealGoldens$' -update-cov-real, then review the diff of testdata/cov/real/"

// covRecorded is testdata/cov/real/recorded.json.
type covRecorded struct {
	Version     string         `json:"version"`
	CorpusStamp string         `json:"corpus_stamped_utc"`
	CodeSHA256  string         `json:"code_sha256"`
	RC          map[string]int `json:"rc"`
}

// covCodeDigest is the sha256 over "<file>\0<sha256 of its bytes>\n" for each covCodeFiles entry.
func covCodeDigest(t *testing.T) string {
	t.Helper()
	files := append([]string(nil), covCodeFiles...)
	sort.Strings(files)
	h := sha256.New()
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("coverage code: %v", err)
		}
		sum := sha256.Sum256(b)
		fmt.Fprintf(h, "%s\x00%x\n", f, sum)
	}
	return hex.EncodeToString(h.Sum(nil))
}

func readCovRecorded(t *testing.T) covRecorded {
	t.Helper()
	var r covRecorded
	data, err := os.ReadFile(filepath.Join(covRealDir, "recorded.json"))
	if err == nil {
		err = json.Unmarshal(data, &r)
	}
	if err != nil || r.Version == "" || r.CorpusStamp == "" || r.CodeSHA256 == "" || len(r.RC) == 0 {
		t.Fatalf("testdata/cov/real/recorded.json is missing or incomplete (%v); %s", err, covReRecord)
	}
	return r
}

// TestCoverageRealRecordingMatchesCode needs no corpus: the recording must have been made by the
// report code in this tree, so a change to that code fails here until the goldens are re-recorded.
func TestCoverageRealRecordingMatchesCode(t *testing.T) {
	if *updateCovReal {
		t.Skip("re-recording")
	}
	if got, rec := covCodeDigest(t), readCovRecorded(t); got != rec.CodeSHA256 {
		t.Fatalf("the coverage report code (%s) changed since the real-corpus goldens were recorded "+
			"(code_sha256 %s, now %s); %s", strings.Join(covCodeFiles, ", "), rec.CodeSHA256, got, covReRecord)
	}
}

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

// corpusStamp is the stamped_utc of root's baseline manifest.
func corpusStamp(t *testing.T, root string) string {
	t.Helper()
	var m struct {
		Stamped string `json:"stamped_utc"`
	}
	data, err := os.ReadFile(filepath.Join(root, "indexes", ".manifest.json"))
	if err == nil {
		err = json.Unmarshal(data, &m)
	}
	if err != nil || m.Stamped == "" {
		t.Fatalf("no readable stamped_utc in %s/indexes/.manifest.json: %v", root, err)
	}
	return m.Stamped
}

// freshDocs is FulltextDocs that never builds: a real corpus must already
// hold an up-to-date index (the test never writes the corpus).
func freshDocs(root string) (map[string]bool, error) {
	path, _ := fts.FreshPath(root, fts.Fingerprint(fts.SourceDirs(root)))
	if path == "" {
		return nil, errors.New("the full-text index of " + root + " is missing or stale")
	}
	return fulltextDocsAt(path)
}

// TestCoverageRealGoldens checks the Go reports of covGoldenVersion against testdata/cov/real/:
// coverage, coverage alarm-rt and, when the install is mounted, coverage --doc-jars. It reads the
// niagara-help checkout named by NIAGARA_HELP_DIR (registry, data root) as data only, so the data
// root's full-text index must be up to date; the listing cache goes to a temporary repository
// holding a copy of the registry. It skips when the variable is unset or with -short; make
// realdata sets it. -update-cov-real rewrites the recording instead of comparing.
func TestCoverageRealGoldens(t *testing.T) {
	help := os.Getenv("NIAGARA_HELP_DIR")
	if help == "" {
		t.Skip("NIAGARA_HELP_DIR is not set: the real coverage goldens need the niagara-help checkout")
	}
	if testing.Short() {
		t.Skip("the real coverage goldens read the real corpus and install")
	}
	root, install, registry := covRegistryEntry(t, help, covGoldenVersion)
	stamp := corpusStamp(t, root)
	rec := covRecorded{Version: covGoldenVersion, CorpusStamp: stamp, CodeSHA256: covCodeDigest(t), RC: map[string]int{}}
	if !*updateCovReal {
		rec = readCovRecorded(t)
		if stamp != rec.CorpusStamp {
			t.Fatalf("the %s corpus was re-stamped at %s since the coverage goldens were recorded on %s; %s",
				covGoldenVersion, stamp, rec.CorpusStamp, covReRecord)
		}
	}
	var b strings.Builder
	res, err := Coverage(&b, root, "")
	if err != nil {
		t.Fatal(err)
	}
	checkCovReal(t, &rec, "coverage", b.String(), min(res.GuideGaps, 1))
	b.Reset()
	if _, err := Coverage(&b, root, "alarm-rt"); err != nil {
		t.Fatal(err)
	}
	checkCovReal(t, &rec, "coverage-module", b.String(), 0)
	if pycompat.IsDir(install) {
		if _, err := freshDocs(root); err != nil {
			t.Fatalf("%v; rebuild it first (nh rebuild-all), this test never writes the corpus", err)
		}
		repo := t.TempDir()
		testkit.WriteFile(t, filepath.Join(repo, "versions", "registry.json"), registry)
		b.Reset()
		rc, err := DocJarCoverage(&b, repo, root, covGoldenVersion, freshDocs)
		if err != nil {
			t.Fatal(err)
		}
		checkCovReal(t, &rec, "coverage-doc-jars", b.String(), rc)
	} else if !*updateCovReal {
		t.Logf("install %s not mounted: coverage --doc-jars not checked", install)
	}
	if *updateCovReal {
		if len(rec.RC) != 3 {
			t.Fatalf("re-recording needs the install mounted (%s) so all three reports are recorded", install)
		}
		data, _ := json.MarshalIndent(rec, "", " ")
		testkit.WriteFile(t, filepath.Join(covRealDir, "recorded.json"), append(data, '\n'))
	}
}

// checkCovReal compares a Go report with its recording, or records it under -update-cov-real.
func checkCovReal(t *testing.T, rec *covRecorded, id, out string, rc int) {
	t.Helper()
	path := filepath.Join(covRealDir, id+".txt")
	if *updateCovReal {
		testkit.WriteFile(t, path, []byte(out))
		rec.RC[id] = rc
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%v; %s", err, covReRecord)
	}
	wantRC, ok := rec.RC[id]
	if !ok {
		t.Fatalf("recorded.json has no exit status for %s; %s", id, covReRecord)
	}
	if out != string(want) || rc != wantRC {
		t.Errorf("%s: rc %d (recorded %d); stdout differs:\n%s", id, rc, wantRC, golden.Diff(want, []byte(out)))
	}
}
