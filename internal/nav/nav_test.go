package nav

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/angeles725/niagara-go/internal/cli"
	"github.com/angeles725/niagara-go/internal/nav/core"
	"github.com/angeles725/niagara-go/internal/nav/registry"
)

func TestEveryOrderedCommandIsRegistered(t *testing.T) {
	if len(Order) != 134 {
		t.Fatalf("Order has %d commands, want 134", len(Order))
	}
	if got := registry.Names(); len(got) != len(Order) {
		t.Fatalf("%d commands registered, want %d", len(got), len(Order))
	}
	if _, err := BuildParser(); err != nil {
		t.Fatal(err)
	}
}

func TestUnportedCommandExitsTwo(t *testing.T) {
	t.Setenv("COLUMNS", "100")
	var out, errb bytes.Buffer
	code := Main([]string{"--base-dir", t.TempDir(), "rank", "BFoo"}, cli.Stdio{Out: &out, Err: &errb})
	if code != cli.ExitCannotSearch || out.Len() != 0 || errb.String() != "mnav: 'rank' is not ported yet\n" {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, out.String(), errb.String())
	}
}

func TestNoBaseDirExitsOne(t *testing.T) {
	t.Chdir(t.TempDir())
	core.Executable = func() (string, error) { return filepath.Join(t.TempDir(), "mnav"), nil }
	t.Cleanup(func() { core.Executable = os.Executable })
	var out, errb bytes.Buffer
	code := Main([]string{"class", "BFoo"}, cli.Stdio{Out: &out, Err: &errb})
	want := "ERROR: Cannot detect base directory.\nUse --base-dir or run from module-navigator/tools/\n"
	if code != cli.ExitNoMatch || out.String() != want || errb.Len() != 0 {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, out.String(), errb.String())
	}
}

func TestEpilogListsEveryCommandOnce(t *testing.T) {
	epilog, err := groupedCommands()
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]int{}
	for _, line := range strings.Split(epilog, "\n") {
		if strings.HasPrefix(line, "    ") {
			seen[strings.Fields(line)[0]]++
		}
	}
	for _, name := range Order {
		if seen[name] != 1 {
			t.Errorf("%s listed %d times", name, seen[name])
		}
	}
}
