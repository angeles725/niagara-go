package nav

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
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
	code := Main([]string{"--base-dir", t.TempDir(), "xref", "BFoo"}, cli.Stdio{Out: &out, Err: &errb})
	if code != cli.ExitCannotSearch || out.Len() != 0 || errb.String() != "mnav: 'xref' is not ported yet\n" {
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

func TestModuleSMANotPortedYet(t *testing.T) {
	var out, errb bytes.Buffer
	code := Main([]string{"--base-dir", t.TempDir(), "module", "alarm-rt", "--sma"}, cli.Stdio{Out: &out, Err: &errb})
	if code != cli.ExitCannotSearch || out.Len() != 0 || errb.String() != "mnav: 'module --sma' is not ported yet\n" {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, out.String(), errb.String())
	}
}

// TestTokenContextParse pins `token --context [N]` (nargs="?", const 2,
// default 0) against the namespaces Python's parser builds.
func TestTokenContextParse(t *testing.T) {
	parser, err := BuildParser()
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		args          []string
		context, n    int
		word          string
		wantUsageExit bool
	}{
		{args: []string{"token", "w", "--context"}, context: 2, n: 50, word: "w"},
		{args: []string{"token", "w", "--context", "5"}, context: 5, n: 50, word: "w"},
		{args: []string{"token", "w"}, context: 0, n: 50, word: "w"},
		{args: []string{"token", "w", "--context", "-n", "3"}, context: 2, n: 3, word: "w"},
		{args: []string{"token", "--context", "w"}, wantUsageExit: true},
	}
	for _, tt := range tests {
		var out, errb bytes.Buffer
		ns, code, done := parser.Parse(tt.args, cli.Stdio{Out: &out, Err: &errb})
		if tt.wantUsageExit {
			if !done || code != 2 || !strings.Contains(errb.String(), "argument --context: invalid int value: 'w'") {
				t.Errorf("%q: done=%v code=%d stderr=%q", tt.args, done, code, errb.String())
			}
			continue
		}
		if done || ns.Int("token_context") != tt.context || ns.Int("n") != tt.n || ns.String("word") != tt.word {
			t.Errorf("%q: context=%d n=%d word=%q done=%v", tt.args, ns.Int("token_context"), ns.Int("n"), ns.String("word"), done)
		}
	}
}

// TestStderrFlushesStdoutFirst: stdout is buffered, but a stderr write first
// flushes it, so a shared terminal sees the order the command wrote.
func TestStderrFlushesStdoutFirst(t *testing.T) {
	var shared bytes.Buffer
	out := bufio.NewWriter(&shared)
	errw := orderedErr{out, &shared}
	fmt.Fprint(out, "out1\n")
	fmt.Fprint(errw, "err1\n")
	fmt.Fprint(out, "out2\n")
	out.Flush()
	if got := shared.String(); got != "out1\nerr1\nout2\n" {
		t.Errorf("shared stream = %q", got)
	}
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errors.New("broken pipe") }

// TestStdoutWriteFailureExitsTwo: a stdout that cannot be written is reported
// on stderr with exit 2, even after a command that found its answer.
func TestStdoutWriteFailureExitsTwo(t *testing.T) {
	t.Setenv("NAV_CORPUS_BASE", filepath.Join("testdata", "fixture", "no-corpus"))
	var errb bytes.Buffer
	code := Main([]string{"--base-dir", filepath.Join("testdata", "fixture", "nav"), "class", "BAlarmService"},
		cli.Stdio{Out: failingWriter{}, Err: &errb})
	if code != cli.ExitCannotSearch || errb.String() != "mnav: cannot write to stdout: broken pipe\n" {
		t.Errorf("exit %d, stderr %q", code, errb.String())
	}
}
