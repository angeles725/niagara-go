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

// TestKeptAndRetiredCover134Commands: the kept commands (Order) and the
// retired ones (Retired) are Python's 134 commands, each once; every kept
// command is registered.
func TestKeptAndRetiredCover134Commands(t *testing.T) {
	if len(Order) != 21 || len(Retired) != 113 {
		t.Fatalf("%d kept and %d retired commands, want 21 and 113", len(Order), len(Retired))
	}
	seen := map[string]bool{}
	for _, n := range append(append([]string(nil), Order...), Retired...) {
		if seen[n] {
			t.Errorf("%s named twice", n)
		}
		seen[n] = true
	}
	for _, n := range Order {
		if _, ok := registry.Lookup(n); !ok {
			t.Errorf("kept command %s is not registered", n)
		}
	}
	if _, err := BuildParser(); err != nil {
		t.Fatal(err)
	}
}

// TestRetiredCommandExitsTwo: a retired command, whatever its arguments and
// however the global options are spelled, prints one line on stderr and
// exits 2; it is in neither the --help list nor the parser's choices.
func TestRetiredCommandExitsTwo(t *testing.T) {
	base := t.TempDir()
	for _, args := range [][]string{
		{"--base-dir", base, "xref", "BFoo"},
		{"-d", base, "ui", "dialogs"},
		{"-d" + base, "method", "--help"},
		{"--base=" + base, "package", "-n", "x"},
		{"--base-d", base, "--", "session"},
		{"repl"},
	} {
		var out, errb bytes.Buffer
		code := Main(args, cli.Stdio{Out: &out, Err: &errb})
		name := args[len(args)-1]
		for i, a := range args {
			if retired[a] {
				name = args[i]
				break
			}
		}
		want := "mnav: '" + name + "' was retired in the Go port (no recorded use); see docs/retired-commands.md\n"
		if code != cli.ExitCannotSearch || out.Len() != 0 || errb.String() != want {
			t.Errorf("%q: code=%d stdout=%q stderr=%q", args, code, out.String(), errb.String())
		}
	}
	var out, errb bytes.Buffer
	if code := Main([]string{"--base-dir", base, "xrefs"}, cli.Stdio{Out: &out, Err: &errb}); code != 2 ||
		strings.Contains(errb.String(), "'xref'") || strings.Contains(errb.String(), "retired") {
		t.Errorf("unknown command: code=%d stderr=%q", code, errb.String())
	}
	epilog, err := groupedCommands()
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range Retired {
		if strings.Contains(epilog, "    "+n+" ") {
			t.Errorf("--help lists retired %s", n)
		}
	}
}

func TestCommandWord(t *testing.T) {
	cases := map[string][]string{
		"class": {"class", "X"}, "stats": {"-d", "/b", "stats"}, "inventory": {"--base-dir=/b", "inventory"},
		"x": {"--", "x"}, "": {"--help", "xref"}, "-": {"-"},
	}
	for want, args := range cases {
		if got := commandWord(args); got != want {
			t.Errorf("commandWord(%q) = %q, want %q", args, got, want)
		}
	}
	for _, args := range [][]string{{"-h", "xref"}, {"--bogus", "xref"}, {}} {
		if got := commandWord(args); got != "" {
			t.Errorf("commandWord(%q) = %q, want none", args, got)
		}
	}
}

// A missing base directory is an input/environment error: the message goes to
// stderr and the exit is 2 (cli.ExitCannotSearch), never 1 ("searched, no match").
func TestNoBaseDirExitsTwoOnStderr(t *testing.T) {
	t.Chdir(t.TempDir())
	t.Setenv("MODULE_NAVIGATOR_DIR", "") // an exported checkout (make realdata) would be found
	core.Executable = func() (string, error) { return filepath.Join(t.TempDir(), "mnav"), nil }
	t.Cleanup(func() { core.Executable = os.Executable })
	var out, errb bytes.Buffer
	code := Main([]string{"class", "BFoo"}, cli.Stdio{Out: &out, Err: &errb})
	want := "ERROR: Cannot detect base directory.\nUse --base-dir or run from module-navigator/tools/\n"
	if code != cli.ExitCannotSearch || errb.String() != want || out.Len() != 0 {
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

// TestMainRejectsBadNavigatorDir: a MODULE_NAVIGATOR_DIR that names no
// checkout is one stderr line naming it and exit 2, not a search elsewhere.
func TestMainRejectsBadNavigatorDir(t *testing.T) {
	bad := filepath.Join(t.TempDir(), "missing")
	t.Setenv("MODULE_NAVIGATOR_DIR", bad)
	var out, errb bytes.Buffer
	code := Main([]string{"class", "BFoo"}, cli.Stdio{Out: &out, Err: &errb})
	want := "mnav: MODULE_NAVIGATOR_DIR=" + bad + ": not a module-navigator checkout (no indexes/ directory)\n"
	if code != cli.ExitCannotSearch || errb.String() != want || out.Len() != 0 {
		t.Errorf("exit %d, stderr %q, stdout %q; want exit 2 and %q", code, errb.String(), out.String(), want)
	}
}
