package nav

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/angeles725/niagara-go/internal/cli"
)

// runAtCase is one command run against a fixture checkout laid out as the
// real one is: <root>/module-navigator/indexes next to <root>/organized.
type runAtCase struct {
	name, indexes, organized string
	args                     []string
}

var runAtCases = []runAtCase{
	{"grep", "search", "search-organized", []string{"grep", "ackAlarm", "-n", "3"}},
	{"resources", "lic", "lic-organized", []string{"resources", "lic-rt"}},
	{"license-feature", "lic", "lic-organized", []string{"license-feature", "LICFEAT"}},
	{"drivers", "drivers", "drivers-organized", []string{"drivers"}},
	{"modules", "nav", "organized", []string{"modules", "--type", "rt"}},
}

type runAtResult struct {
	out, err string
	code     int
}

// runAtCheckout links a fixture's indexes and corpus into a fresh
// <root>/module-navigator + <root>/organized layout and returns the checkout.
func runAtCheckout(t *testing.T, c runAtCase) string {
	t.Helper()
	root := t.TempDir()
	nav := filepath.Join(root, "module-navigator")
	if err := os.Mkdir(nav, 0o755); err != nil {
		t.Fatal(err)
	}
	fixture, err := filepath.Abs(filepath.Join("testdata", "fixture"))
	if err != nil {
		t.Fatal(err)
	}
	links := map[string]string{
		filepath.Join(nav, "indexes"):    filepath.Join(fixture, c.indexes, "indexes"),
		filepath.Join(root, "organized"): filepath.Join(fixture, c.organized),
	}
	for link, target := range links {
		if err := os.Symlink(target, link); err != nil {
			if runtime.GOOS == "windows" {
				t.Skipf("no symlink privilege: %v", err)
			}
			t.Fatal(err)
		}
	}
	return nav
}

func runAtCapture(run func(cli.Stdio) int) runAtResult {
	var out, errb bytes.Buffer
	code := run(cli.Stdio{In: bytes.NewReader(nil), Out: &out, Err: &errb})
	return runAtResult{out.String(), errb.String(), code}
}

// TestRunAtMatchesMain: RunAt pinned to a checkout, from an unrelated working
// directory with no MODULE_NAVIGATOR_DIR, NAV_DIR or NAV_CORPUS_BASE, prints
// exactly what Main prints when MODULE_NAVIGATOR_DIR names that checkout -
// including the commands that read the organized/ corpus next to it. Then
// every case runs again concurrently, each against its own checkout, and
// still gives the same result.
func TestRunAtMatchesMain(t *testing.T) {
	future := time.Now().Add(time.Hour)
	for _, base := range []string{"drivers", "search"} {
		ci := filepath.Join("testdata", "fixture", base, "indexes", "class-index.json")
		if err := os.Chtimes(ci, future, future); err != nil {
			t.Fatal(err)
		}
	}
	navDirs := make([]string, len(runAtCases))
	want := make([]runAtResult, len(runAtCases))
	for i, c := range runAtCases {
		navDirs[i] = runAtCheckout(t, c)
	}
	t.Chdir(t.TempDir())
	t.Setenv("NAV_DIR", "")
	t.Setenv("NAV_CORPUS_BASE", "")
	for i, c := range runAtCases {
		t.Setenv("MODULE_NAVIGATOR_DIR", navDirs[i])
		want[i] = runAtCapture(func(s cli.Stdio) int { return Main(c.args, s) })
		if want[i].code != cli.ExitFound {
			t.Fatalf("%s: Main exit %d, want %d (fixture layout broken?)\nstderr: %s",
				c.name, want[i].code, cli.ExitFound, want[i].err)
		}
	}
	t.Setenv("MODULE_NAVIGATOR_DIR", "")
	for i, c := range runAtCases {
		got := runAtCapture(func(s cli.Stdio) int { return RunAt(navDirs[i], c.args, s) })
		if got != want[i] {
			t.Errorf("%s: RunAt = %+v\nwant %+v", c.name, got, want[i])
		}
	}
	got := make([]runAtResult, len(runAtCases))
	var wg sync.WaitGroup
	for round := 0; round < 4; round++ {
		for i, c := range runAtCases {
			wg.Add(1)
			go func() {
				defer wg.Done()
				r := runAtCapture(func(s cli.Stdio) int { return RunAt(navDirs[i], c.args, s) })
				if round == 0 {
					got[i] = r
				} else if r != want[i] {
					t.Errorf("%s (concurrent round %d): RunAt = %+v\nwant %+v", c.name, round, r, want[i])
				}
			}()
		}
	}
	wg.Wait()
	for i, c := range runAtCases {
		if got[i] != want[i] {
			t.Errorf("%s (concurrent): RunAt = %+v\nwant %+v", c.name, got[i], want[i])
		}
	}
}

// TestRunAtBaseDirWins: an explicit --base-dir in the arguments still names
// the indexes, as with Main; the corpus stays the pinned checkout's.
func TestRunAtBaseDirWins(t *testing.T) {
	c := runAtCases[0]
	nav := runAtCheckout(t, c)
	t.Chdir(t.TempDir())
	t.Setenv("MODULE_NAVIGATOR_DIR", "")
	t.Setenv("NAV_DIR", "")
	t.Setenv("NAV_CORPUS_BASE", "")
	empty := t.TempDir()
	r := runAtCapture(func(s cli.Stdio) int { return RunAt(empty, append([]string{"--base-dir", nav}, c.args...), s) })
	if r.code != cli.ExitCannotSearch || !bytes.Contains([]byte(r.err), []byte(filepath.Join(filepath.Dir(empty), "organized"))) {
		t.Errorf("RunAt(empty checkout, --base-dir) = %+v, want exit 2 naming the pinned corpus", r)
	}
	r = runAtCapture(func(s cli.Stdio) int { return RunAt(nav, append([]string{"--base-dir", empty}, "modules"), s) })
	if r.code != cli.ExitCannotSearch {
		t.Errorf("RunAt(--base-dir with no indexes) = %+v, want exit 2", r)
	}
}
