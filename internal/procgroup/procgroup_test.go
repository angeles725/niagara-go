package procgroup

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// helperEnv makes the test binary act as a child of the tests instead of
// running them; its value is the mode (see runHelper).
const helperEnv = "PROCGROUP_TEST_HELPER"

// pidFileEnv is where a forking helper writes "<own pid> <grandchild pid>".
const pidFileEnv = "PROCGROUP_TEST_PIDFILE"

func TestMain(m *testing.M) {
	if mode := os.Getenv(helperEnv); mode != "" {
		os.Exit(runHelper(mode))
	}
	os.Exit(m.Run())
}

// runHelper is the test binary as a child:
//
//	sleep    sleeps 30 s
//	fork     starts a "sleep" grandchild (no group of its own), writes the
//	         pid file and waits for it, like a wrapper that does not exec
//	exit     writes "boom" to stderr and "out" to stdout, exits 7
//	echo     copies stdin lines to stdout (a backend over pipes); with
//	         PROCGROUP_TEST_FORK=1 it forks a grandchild first, as "fork"
//	runner   the process under test of the signal tests (runner, unix)
func runHelper(mode string) int {
	switch mode {
	case "sleep":
		time.Sleep(30 * time.Second)
		return 0
	case "fork":
		g, err := forkGrandchild()
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 4
		}
		_ = g.Wait()
		return 0
	case "exit":
		fmt.Fprint(os.Stdout, "out")
		fmt.Fprint(os.Stderr, "boom")
		return 7
	case "echo":
		if os.Getenv("PROCGROUP_TEST_FORK") == "1" {
			if _, err := forkGrandchild(); err != nil {
				fmt.Fprintln(os.Stderr, err)
				return 4
			}
		}
		sc := bufio.NewScanner(os.Stdin)
		for sc.Scan() {
			fmt.Println(sc.Text())
		}
		return 0
	case "runner":
		return runner()
	}
	fmt.Fprintf(os.Stderr, "unknown helper mode %q\n", mode)
	return 4
}

// forkGrandchild starts the test binary in "sleep" mode and writes the pid
// file; the grandchild keeps the inherited stdout and stderr.
func forkGrandchild() (*exec.Cmd, error) {
	self, err := os.Executable()
	if err != nil {
		return nil, err
	}
	g := exec.Command(self)
	g.Env = append(os.Environ(), helperEnv+"=sleep")
	g.Stdout, g.Stderr = os.Stdout, os.Stderr
	if err := g.Start(); err != nil {
		return nil, err
	}
	line := fmt.Sprintf("%d %d\n", os.Getpid(), g.Process.Pid)
	if err := os.WriteFile(os.Getenv(pidFileEnv)+".tmp", []byte(line), 0o644); err != nil {
		return nil, err
	}
	return g, os.Rename(os.Getenv(pidFileEnv)+".tmp", os.Getenv(pidFileEnv))
}

// helper is a *Cmd that runs the test binary in mode.
func helper(t *testing.T, ctx context.Context, mode string, env ...string) *Cmd {
	t.Helper()
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	c := CommandContext(ctx, self)
	c.Env = append(append(os.Environ(), helperEnv+"="+mode), env...)
	return c
}

// A child's own outcome passes through: its exit status, and its stderr in
// the *exec.ExitError of Output.
func TestRunPassesTheChildOutcome(t *testing.T) {
	out, err := helper(t, context.Background(), "exit").Output()
	var ee *exec.ExitError
	if !errors.As(err, &ee) || ee.ExitCode() != 7 || string(ee.Stderr) != "boom" || string(out) != "out" {
		t.Fatalf("Output = %q, %v (stderr %q), want exit 7 with out/boom", out, err, stderrOf(ee))
	}
	both, err := helper(t, context.Background(), "exit").CombinedOutput()
	if !errors.As(err, &ee) || ee.ExitCode() != 7 || !strings.Contains(string(both), "boom") || !strings.Contains(string(both), "out") {
		t.Fatalf("CombinedOutput = %q, %v", both, err)
	}
}

func stderrOf(ee *exec.ExitError) []byte {
	if ee == nil {
		return nil
	}
	return ee.Stderr
}

// A program that does not start returns exec's error, not an interrupt,
// and a second run of the same Cmd is refused as exec refuses it.
func TestStartFailure(t *testing.T) {
	c := CommandContext(context.Background(), "procgroup-no-such-program")
	err := c.Run()
	var in *InterruptError
	if !errors.Is(err, exec.ErrNotFound) || errors.As(err, &in) {
		t.Fatalf("Run = %v, want exec.ErrNotFound", err)
	}
	if err := helper(t, context.Background(), "sleep").Wait(); err == nil {
		t.Fatal("Wait before Start succeeded")
	}
}

// The timeout ends a running child promptly; the caller's own context says
// it was the timeout.
func TestTimeoutStopsTheChild(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	t0 := time.Now()
	err := helper(t, ctx, "sleep").Run()
	if err == nil || ctx.Err() == nil {
		t.Fatalf("Run = %v, ctx %v: want a killed child after the timeout", err, ctx.Err())
	}
	if d := time.Since(t0); d > 10*time.Second {
		t.Fatalf("Run took %s", d)
	}
}

// A long-running backend over pipes (the searchdoor shape): it answers
// while it runs, and the caller's cancel ends it.
func TestBackendOverPipes(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	c := helper(t, ctx, "echo")
	in, out := pipes(t, c)
	if err := c.Start(); err != nil {
		t.Fatal(err)
	}
	ask(t, in, out, "ping")
	cancel()
	if err := c.Wait(); err == nil {
		t.Fatal("Wait = nil after the cancel")
	}
}

// pipes sets up the stdin and stdout pipes of a backend; the child's
// stderr goes to the test's.
func pipes(t *testing.T, c *Cmd) (io.WriteCloser, *bufio.Reader) {
	t.Helper()
	in, err := c.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	out, err := c.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	c.Stderr = os.Stderr
	return in, bufio.NewReader(out)
}

// ask writes a line to the backend and checks it comes back.
func ask(t *testing.T, in io.Writer, out *bufio.Reader, line string) {
	t.Helper()
	if _, err := fmt.Fprintln(in, line); err != nil {
		t.Fatal(err)
	}
	got, err := out.ReadString('\n')
	if strings.TrimRight(got, "\r\n") != line || err != nil {
		t.Fatalf("backend answered %q, %v; want %q", got, err, line)
	}
}

func TestInterruptErrorText(t *testing.T) {
	if got := (&InterruptError{os.Interrupt}).Error(); got != "interrupted by interrupt" {
		t.Fatalf("Error() = %q", got)
	}
}
