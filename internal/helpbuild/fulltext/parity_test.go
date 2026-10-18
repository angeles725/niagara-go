package fulltext

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/angeles725/niagara-go/internal/help/fts"
)

var update = flag.Bool("update", false, "regenerate testdata/synthetic.dump with the Python builder")

// hideRoot rewrites the temp root in the stored fingerprint.
func hideRoot(root string) func(string) string {
	return func(s string) string { return strings.ReplaceAll(s, root, "$ROOT") }
}

const syntheticDump = "testdata/synthetic.dump"

func TestBuildMatchesPythonOnSyntheticTree(t *testing.T) {
	root := writeTree(t)
	if *update {
		py := filepath.Join(t.TempDir(), "python.sqlite")
		if err := pythonBuild(t, root, py); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(syntheticDump, []byte(dumpText(py, hideRoot(root))), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(syntheticDump)
	if err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "out", fts.DBName)
	res, err := Build(root, Options{Output: out, Workers: 3})
	if err != nil {
		t.Fatal(err)
	}
	got := dumpText(out, hideRoot(root))
	if runtime.GOOS == "windows" {
		// Captured on Linux: the fingerprint row holds the native-separator source paths and
		// the file system's mtime resolution (100 ns on NTFS); every other record must match.
		// The row is still the reader's fingerprint (checked below).
		want, got = []byte(dropFingerprint(string(want))), dropFingerprint(got)
	}
	if got != string(want) {
		_, diffs := compareDumps(lines(string(want)), lines(got), 10)
		t.Fatalf("Go dump differs from the Python one:\n%s", strings.Join(diffs, "\n"))
	}
	if res.Fingerprint != fts.Fingerprint(fts.SourceDirs(root)) {
		t.Errorf("Result.Fingerprint is not the reader's fingerprint")
	}
	if n := strings.Count(got, "\ndocs\t"); res.Documents != n {
		t.Errorf("Documents = %d, the dump has %d docs rows", res.Documents, n)
	}
	wantLog := fmt.Sprintf("full-text index: %d documents -> %s", res.Documents, out)
	if len(res.Log) != 1 || res.Log[0] != wantLog {
		t.Errorf("Log = %q, want [%q]", res.Log, wantLog)
	}
}

// dropFingerprint removes the meta fingerprint record from a dump.
func dropFingerprint(dump string) string {
	var b strings.Builder
	for _, l := range strings.SplitAfter(dump, "\n") {
		if !strings.HasPrefix(l, "meta\t'fingerprint'\t") {
			b.WriteString(l)
		}
	}
	return b.String()
}

func lines(s string) func(func(string) bool) {
	return func(yield func(string) bool) {
		for _, l := range strings.SplitAfter(s, "\n") {
			if l != "" && !yield(strings.TrimSuffix(l, "\n")) {
				return
			}
		}
	}
}

// pythonBuild runs fulltext_index.build(root, path=out, quiet=True) from the
// niagara-help checkout ($NIAGARA_HELP_DIR, else
// $NIAGARA_RESEARCH/niagara-help); it never writes into that checkout.
func pythonBuild(t *testing.T, root, out string) error {
	t.Helper()
	python, err := exec.LookPath("python3")
	if err != nil {
		t.Skip("python3 not on PATH")
	}
	tools := filepath.Join(niagaraHelpDir(), "tools")
	if _, err := os.Stat(filepath.Join(tools, "niagara_help_lib", "fulltext_index.py")); err != nil {
		t.Skipf("niagara-help tools not available: %v", err)
	}
	cmd := exec.Command(python, "-I", "-B", "-c",
		"import sys; sys.path.insert(0, sys.argv[1]); "+
			"from niagara_help_lib import fulltext_index as f; "+
			"f.build(sys.argv[2], path=sys.argv[3], quiet=True)",
		tools, root, out)
	if b, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("python build %s: %v\n%s", root, err, b)
	}
	return nil
}

func niagaraHelpDir() string {
	if d := os.Getenv("NIAGARA_HELP_DIR"); d != "" {
		return d
	}
	root := os.Getenv("NIAGARA_RESEARCH")
	if root == "" {
		root = "/home/cristian/niagara-research"
	}
	return filepath.Join(root, "niagara-help")
}

func TestBuildIsTheSameForAnyWorkerCount(t *testing.T) {
	root := writeBigTree(t, 600) // several 256-file batches
	var dumps []string
	for _, n := range []int{1, 7} {
		out := filepath.Join(t.TempDir(), fts.DBName)
		if _, err := Build(root, Options{Output: out, Workers: n}); err != nil {
			t.Fatal(err)
		}
		dumps = append(dumps, dumpText(out, hideRoot(root)))
	}
	if dumps[0] != dumps[1] {
		_, diffs := compareDumps(lines(dumps[0]), lines(dumps[1]), 5)
		t.Fatalf("1 and 7 workers build different indexes:\n%s", strings.Join(diffs, "\n"))
	}
}
