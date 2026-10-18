package help

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/angeles725/niagara-go/internal/cli"
	"github.com/angeles725/niagara-go/internal/pyref"
)

// rebuildParityVar selects the real versions of TestParityRebuildReal (a
// path-list of registered version ids of the niagara-help checkout).
const rebuildParityVar = "NH_PARITY_REBUILD"

// rebuildDryCases run on the live data root: a dry run writes nothing.
var rebuildDryCases = [][]string{
	{"rebuild-all", "--help"},
	{"rebuild-all", "--dry-run"},
	{"rebuild-all", "--only", "build_", "--dry-run"},
	{"rebuild-all", "--dry-run", "--only", "zzz"},
}

// rebuildCopyCases run on a copy of the data root's devguide/ and indexes/
// (a real build of the class indexes, then a stamp).
var rebuildCopyCases = [][]string{
	{"rebuild-all", "--only", "build_class"},
	{"rebuild-all", "--stamp-only"},
}

// rebuildOutputs are the files a copy case may rewrite, compared after
// each side (the manifest without its stamped_utc).
var rebuildOutputs = []string{"indexes/class-index.json", "indexes/jdk-class-index.json", "indexes/.manifest.json"}

var (
	rebuildTimes   = regexp.MustCompile(`\d+(\.\d+)?\s?(ms|s|sec|secs|seconds)\b`)
	rebuildStamped = regexp.MustCompile(`\d{4}-\d\d-\d\dT\d\d:\d\d:\d\d(\.\d+)?\+00:00`)
)

// normRebuild folds what legitimately differs between two runs: elapsed
// times and stamp times. On Python's side it also drops the
// build_call_index.py row, which the Go plan no longer has (decision I3-5).
func normRebuild(s string, python bool) string {
	if python {
		var keep []string
		for _, l := range strings.SplitAfter(s, "\n") {
			if !strings.Contains(l, "build_call_index.py") {
				keep = append(keep, l)
			}
		}
		s = strings.Join(keep, "")
	}
	return rebuildStamped.ReplaceAllString(rebuildTimes.ReplaceAllString(s, "<T>"), "<STAMP>")
}

// TestParityRebuildReal compares nh rebuild-all with the pinned
// niagara_help.py: the dry runs on the live data root (nothing is
// written), then a real class-index build and a stamp on a copy of the
// data root's devguide/ and indexes/ in a temporary folder under $HOME,
// removed afterwards. stdout, stderr and the exit status are compared
// after normRebuild, and the rewritten indexes and manifest byte for byte.
func TestParityRebuildReal(t *testing.T) {
	list := os.Getenv(rebuildParityVar)
	if list == "" {
		t.Skipf("real-data parity runs only on request: set %s=<version id>[:...]", rebuildParityVar)
	}
	tools := filepath.Join(pyref.Dir(t, "niagara-help"), "tools")
	python := pyref.Python(t)
	help := os.Getenv(EnvDir)
	home, _ := os.UserHomeDir()
	if help == "" {
		help = filepath.Join(home, "niagara-research", "niagara-help")
	}
	for _, vid := range filepath.SplitList(list) {
		if vid == "" {
			continue
		}
		t.Run(vid, func(t *testing.T) {
			repo, base, _ := maintParityRepo(t, help, vid)
			for _, args := range rebuildDryCases {
				compareRebuild(t, python, tools, repo, vid, args, "", nil)
			}
			scratch, err := os.MkdirTemp(home, "nh-rebuild-parity-")
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { os.RemoveAll(scratch) })
			copyRoot := filepath.Join(scratch, vid)
			for _, dir := range []string{"devguide", "indexes"} {
				copyCorpusTree(t, filepath.Join(base, dir), filepath.Join(copyRoot, dir))
			}
			copyRepo := t.TempDir()
			pointRegistry(t, repo, copyRepo, vid, copyRoot)
			for _, args := range rebuildCopyCases {
				compareRebuild(t, python, tools, copyRepo, vid, args, copyRoot, rebuildOutputs)
			}
		})
	}
}

// compareRebuild runs args on both sides; with outputs, the files Python
// left under root are compared with Go's.
func compareRebuild(t *testing.T, python, tools, repo, vid string, args []string, root string, outputs []string) {
	t.Helper()
	argv := append([]string{"--niagara-version", vid}, args...)
	pyOut, pyErr, pyCode := runPython(t, python, tools, repo, argv)
	pyFiles := snapshot(t, root, outputs)
	t.Setenv(EnvDir, repo)
	t.Setenv(envVersionForTest, "")
	t.Setenv("COLUMNS", "100")
	var out, errb bytes.Buffer
	code := Main(argv, cli.Stdio{In: strings.NewReader(""), Out: &out, Err: &errb})
	goFiles := snapshot(t, root, outputs)
	what := strings.Join(args, " ")
	compareMaint(t, what,
		maintOutcome{stdout: normRebuild(pyOut, true), stderr: normRebuild(pyErr, true), code: pyCode},
		maintOutcome{stdout: normRebuild(out.String(), false), stderr: normRebuild(errb.String(), false), code: code})
	for _, rel := range outputs {
		if pyFiles[rel] != goFiles[rel] {
			t.Errorf("%s: %s differs (python %d bytes, go %d)", what, rel, len(pyFiles[rel]), len(goFiles[rel]))
		}
	}
	t.Logf("%s: exit %d, %d stdout lines", what, code, strings.Count(out.String(), "\n"))
}

// snapshot reads outputs under root; the manifest's stamped_utc is folded.
func snapshot(t *testing.T, root string, outputs []string) map[string]string {
	t.Helper()
	got := map[string]string{}
	for _, rel := range outputs {
		body, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil {
			t.Fatalf("%s: %v", rel, err)
		}
		got[rel] = rebuildStamped.ReplaceAllString(string(body), "<STAMP>")
	}
	return got
}

// pointRegistry writes into dst the registry of src with vid's data root
// replaced by root.
func pointRegistry(t *testing.T, src, dst, vid, root string) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(src, "versions", "registry.json"))
	if err != nil {
		t.Fatal(err)
	}
	var reg map[string]any
	if err := json.Unmarshal(data, &reg); err != nil {
		t.Fatal(err)
	}
	reg["versions"].(map[string]any)[vid].(map[string]any)["data_root"] = root
	out, err := json.MarshalIndent(reg, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(dst, "versions", "registry.json"), string(out))
}

// copyCorpusTree copies the regular files under src to dst, skipping the
// full-text database and the retired call index (no builder of the copy
// cases reads them).
func copyCorpusTree(t *testing.T, src, dst string) {
	t.Helper()
	err := filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, path)
		target := filepath.Join(dst, rel)
		switch {
		case info.IsDir():
			return os.MkdirAll(target, 0o755)
		case !info.Mode().IsRegular(), info.Name() == "fulltext.sqlite", info.Name() == "call-index.json":
			return nil
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, body, 0o644)
	})
	if err != nil {
		t.Fatal(err)
	}
}
