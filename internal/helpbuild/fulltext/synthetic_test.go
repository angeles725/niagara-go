package fulltext

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/angeles725/niagara-go/internal/help/fts"
)

var update = flag.Bool("update", false, "rewrite testdata/synthetic.dump from the Go build (review the diff)")

// hideRoot rewrites the temp root in the stored fingerprint.
func hideRoot(root string) func(string) string {
	return func(s string) string { return strings.ReplaceAll(s, root, "$ROOT") }
}

const syntheticDump = "testdata/synthetic.dump"

// TestBuildMatchesSyntheticDump compares the canonical dump of the Go build
// of the synthetic tree with testdata/synthetic.dump, the dump of the Python
// builder's index of the same tree (frozen when D47 retired the Python
// reference); -update rewrites it from the Go build.
func TestBuildMatchesSyntheticDump(t *testing.T) {
	root := writeTree(t)
	out := filepath.Join(t.TempDir(), "out", fts.DBName)
	res, err := Build(root, Options{Output: out, Workers: 3})
	if err != nil {
		t.Fatal(err)
	}
	got := dumpText(out, hideRoot(root))
	if *update {
		if err := os.WriteFile(syntheticDump, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(syntheticDump)
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS == "windows" {
		// Captured on Linux: the fingerprint row holds the native-separator source paths and
		// the file system's mtime resolution (100 ns on NTFS); every other record must match.
		// The row is still the reader's fingerprint (checked below).
		want, got = []byte(dropFingerprint(string(want))), dropFingerprint(got)
	}
	if got != string(want) {
		_, diffs := compareDumps(lines(string(want)), lines(got), 10)
		t.Fatalf("Go dump differs from testdata/synthetic.dump:\n%s", strings.Join(diffs, "\n"))
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
