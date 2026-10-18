package golden

import (
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"testing"

	"github.com/angeles725/niagara-go/internal/cli"
)

func TestRunGoPassesGoOnlyCases(t *testing.T) {
	t.Setenv(UpdateEnv, "")
	RunGo(t, toy)
}

// TestRunGoUpdateRewritesOutputs regenerates a suite given by a relative path: the per-case
// working directory of the rewrite must not leak into the check that follows it.
func TestRunGoUpdateRewritesOutputs(t *testing.T) {
	t.Setenv(UpdateEnv, "1")
	t.Setenv(BinEnv, "")
	parent := t.TempDir()
	suite := filepath.Join(parent, "suite", "golden")
	stale := filepath.Join(suite, "stale")
	fresh := filepath.Join(suite, "fresh")
	for _, d := range []string{stale, fresh} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	write(t, filepath.Join(stale, "cmd.json"), `{"args":["--tmp","{tmp}/x"]}`)
	write(t, filepath.Join(stale, "stdout"), "old\n")
	write(t, filepath.Join(stale, "stderr"), "old warning\n")
	write(t, filepath.Join(stale, "exit"), "9\n")
	write(t, filepath.Join(fresh, "cmd.json"), `{"args":["--crash"]}`) // a new case: cmd.json only
	t.Chdir(parent)

	runGo(t, "suite", toy)

	if wd, err := os.Getwd(); err != nil || wd != parent {
		t.Fatalf("cwd after runGo = %q (%v), want %q", wd, err, parent)
	}
	want := map[string]string{
		"stale/stdout": "wrote {tmp}/x\n",
		"stale/exit":   strconv.Itoa(cli.ExitFound) + "\n",
		"fresh/stdout": "",
		"fresh/stderr": "toy: boom\n",
		"fresh/exit":   strconv.Itoa(cli.ExitCannotSearch) + "\n",
	}
	for name, body := range want {
		got, err := os.ReadFile(filepath.Join(suite, name))
		if err != nil || string(got) != body {
			t.Errorf("%s = %q (%v), want %q", name, got, err, body)
		}
	}
	if _, err := os.Stat(filepath.Join(stale, "stderr")); !os.IsNotExist(err) {
		t.Errorf("an empty stderr left the stale stderr file (stat err %v)", err)
	}
}

func TestUpdatingFollowsTheVariable(t *testing.T) {
	t.Setenv(UpdateEnv, "")
	if Updating() {
		t.Fatal("Updating with an empty GOLDEN_UPDATE")
	}
	t.Setenv(UpdateEnv, "1")
	if !Updating() {
		t.Fatal("not Updating with GOLDEN_UPDATE=1")
	}
}

func TestRewriteOutputsReportsChangedFiles(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "stdout"), "same\n")
	write(t, filepath.Join(dir, "stderr"), "old\n")
	write(t, filepath.Join(dir, "exit"), "0\n")

	got, err := rewriteOutputs(dir, result{stdout: []byte("same\n"), stderr: []byte("new\n"), exit: 0})
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{filepath.Join(dir, "stderr")}; !slices.Equal(got, want) {
		t.Errorf("changed = %v, want %v", got, want)
	}
	got, err = rewriteOutputs(dir, result{stdout: []byte("same\n"), exit: 0}) // empty stderr removes it
	if err != nil || !slices.Equal(got, []string{filepath.Join(dir, "stderr")}) {
		t.Errorf("removal: changed = %v (%v)", got, err)
	}
	if got, err = rewriteOutputs(dir, result{stdout: []byte("same\n")}); err != nil || len(got) != 0 {
		t.Errorf("unchanged rewrite reported %v (%v)", got, err)
	}
}
