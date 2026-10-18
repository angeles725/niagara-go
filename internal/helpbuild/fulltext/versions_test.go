package fulltext

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/angeles725/niagara-go/internal/help/fts"
	"github.com/angeles725/niagara-go/internal/index"
)

// TestBuildMatchesPythonOnRegisteredVersions builds every registered version
// of the niagara-help checkout at $NIAGARA_HELP_DIR (the live data: registry,
// versions/) twice, with the frozen Python builder (pythonTools: the
// pre-go-retire worktree, D47) and with Build, into temp files (the checkout's
// own indexes are never written) and requires equal canonical dumps. It is slow (minutes per
// version): run it with -parallel 2 on a busy host. Both build times are
// logged.
func TestBuildMatchesPythonOnRegisteredVersions(t *testing.T) {
	repo := os.Getenv("NIAGARA_HELP_DIR")
	if repo == "" {
		t.Skip("NIAGARA_HELP_DIR not set: real-version parity not checked")
	}
	if testing.Short() {
		t.Skip("real-version parity is slow")
	}
	pythonTools(t) // a missing reference stops the test before any build
	reg, err := index.LoadRegistry(repo)
	if err != nil || reg == nil {
		t.Fatalf("no registry under %s: %v", repo, err)
	}
	ids := append([]string(nil), reg.Versions.Keys()...)
	sort.Strings(ids)
	for _, vid := range ids {
		t.Run(vid, func(t *testing.T) {
			t.Parallel()
			root, err := index.DataRoot(repo, vid)
			if err != nil {
				t.Fatal(err)
			}
			dir := t.TempDir()
			py, goDB := filepath.Join(dir, "python.sqlite"), filepath.Join(dir, "go", fts.DBName)

			start := time.Now()
			if err := pythonBuild(t, root, py); err != nil {
				t.Fatal(err)
			}
			pyTime := time.Since(start)
			start = time.Now()
			res, err := Build(root, Options{Output: goDB})
			if err != nil {
				t.Fatal(err)
			}
			goTime := time.Since(start)
			t.Logf("%s: %d documents from %d files (%s); python %s, go %s",
				vid, res.Documents, res.Files, strings.Join(res.Sources, ","),
				pyTime.Round(100*time.Millisecond), goTime.Round(100*time.Millisecond))

			n, diffs := compareDumps(dump(py, nil), dump(goDB, nil), 20)
			if len(diffs) > 0 {
				t.Fatalf("%d records compared; the Go index differs from Python's:\n%s", n, strings.Join(diffs, "\n"))
			}
			t.Logf("%s: %d dump records equal", vid, n)
			if keep := os.Getenv("FULLTEXT_KEEP_DIR"); keep != "" {
				keepCopies(t, keep, vid, py, goDB)
			}
		})
	}
}

// keepCopies copies both databases to <keep>/<version>/{python,go}.sqlite
// for a later behavioural check (FULLTEXT_KEEP_DIR); a copy, not a rename,
// because the keep directory may be on another file system than TMPDIR.
func keepCopies(t *testing.T, keep, vid, py, goDB string) {
	t.Helper()
	dst := filepath.Join(keep, vid)
	if err := os.MkdirAll(dst, 0o755); err != nil {
		t.Fatal(err)
	}
	for src, name := range map[string]string{py: "python.sqlite", goDB: "go.sqlite"} {
		data, err := os.ReadFile(src)
		if err == nil {
			err = os.WriteFile(filepath.Join(dst, name), data, 0o644)
		}
		if err != nil {
			t.Fatal(err)
		}
	}
}
