//go:build unix

package procgroup

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// A wrapper that forks its real program leaves nothing running after the
// timeout: the whole process group is killed, the grandchild too.
func TestTimeoutKillsTheGroup(t *testing.T) {
	needProc(t)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	c, pidFile := forking(t, ctx)
	if err := c.Start(); err != nil {
		t.Fatal(err)
	}
	child, grand, ok := waitPids(ctx, pidFile)
	err := c.Wait()
	if !ok {
		t.Skip("the forking helper did not write its pids before the timeout")
	}
	if err == nil || ctx.Err() == nil {
		t.Fatalf("Wait = %v, ctx %v: want the timeout", err, ctx.Err())
	}
	waitGone(t, child)
	waitGone(t, grand)
}

// The caller's cancel kills the whole group as the timeout does.
func TestCancelKillsTheGroup(t *testing.T) {
	needProc(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	c, pidFile := forking(t, ctx)
	if err := c.Start(); err != nil {
		t.Fatal(err)
	}
	child, grand, ok := waitPids(deadline(t, 10*time.Second), pidFile)
	cancel()
	if err := c.Wait(); err == nil {
		t.Fatal("Wait = nil after the cancel")
	}
	if !ok {
		t.Fatal("the forking helper never wrote its pids")
	}
	waitGone(t, child)
	waitGone(t, grand)
}

// A forking backend over pipes is killed whole by the cancel; the
// grandchild holding the stdout pipe does not keep Wait waiting.
func TestBackendCancelKillsTheGroup(t *testing.T) {
	needProc(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	pidFile := filepath.Join(t.TempDir(), "pids")
	c := helper(t, ctx, "echo", pidFileEnv+"="+pidFile, "PROCGROUP_TEST_FORK=1")
	c.WaitDelay = 5 * time.Second
	in, out := pipes(t, c)
	if err := c.Start(); err != nil {
		t.Fatal(err)
	}
	ask(t, in, out, "ping")
	child, grand, ok := waitPids(deadline(t, 10*time.Second), pidFile)
	if !ok {
		t.Fatal("the backend never wrote its pids")
	}
	cancel()
	t0 := time.Now()
	if err := c.Wait(); err == nil {
		t.Fatal("Wait = nil after the cancel")
	}
	if d := time.Since(t0); d > 3*time.Second {
		t.Errorf("Wait took %s: the grandchild kept the pipe open", d)
	}
	waitGone(t, child)
	waitGone(t, grand)
}

// SIGINT to this process while the child runs kills the child's group,
// which does not get the terminal's Ctrl-C itself, and this process then
// dies of SIGINT as it would by default (130).
func TestInterruptKillsTheGroupAndReraises(t *testing.T) { diesOf(t, syscall.SIGINT) }

// SIGTERM does the same, and this process dies of SIGTERM (143).
func TestTerminateKillsTheGroupAndReraises(t *testing.T) { diesOf(t, syscall.SIGTERM) }

func diesOf(t *testing.T, sig syscall.Signal) {
	r := startRunner(t, "")
	if err := r.cmd.Process.Signal(sig); err != nil {
		t.Fatal(err)
	}
	err := r.cmd.Wait()
	var ee *exec.ExitError
	if !errors.As(err, &ee) {
		t.Fatalf("runner ended with %v, want death by %s", err, sig)
	} else if ws := ee.Sys().(syscall.WaitStatus); !ws.Signaled() || ws.Signal() != sig {
		t.Fatalf("runner ended with %v, want death by %s\n%s", ee, sig, r.out)
	}
	waitGone(t, r.child)
	waitGone(t, r.grand)
}

// A caller that catches SIGINT itself and leaves ReturnInterrupt false
// outlives the re-raised signal and gets the *InterruptError back at once,
// not after a fixed wait. Its handler sees the press twice (the original
// and the re-raise): the documented reason for ReturnInterrupt.
func TestCaughtInterruptReturnsPromptly(t *testing.T) {
	r := startRunner(t, "catch")
	res := r.interrupt(t)
	if res.sig != "interrupt" || res.count != 2 {
		t.Errorf("runner: %+v, want the SIGINT returned and seen twice by its handler\n%s", res, r.out)
	}
	if res.lag > 500*time.Millisecond {
		t.Errorf("Wait returned %s after the signal, want at once", res.lag)
	}
	waitGone(t, r.child)
	waitGone(t, r.grand)
}

// With ReturnInterrupt the caller's handler sees the press once (no
// re-raise) and Wait returns the *InterruptError at once.
func TestReturnInterruptDoesNotReraise(t *testing.T) {
	r := startRunner(t, "return")
	res := r.interrupt(t)
	if res.sig != "interrupt" || res.count != 1 {
		t.Errorf("runner: %+v, want the SIGINT returned and seen once by its handler\n%s", res, r.out)
	}
	if res.lag > 500*time.Millisecond {
		t.Errorf("Wait returned %s after the signal, want at once", res.lag)
	}
	waitGone(t, r.child)
	waitGone(t, r.grand)
}

// runnerEnv holds the runner's handling: "" (none of its own), "catch"
// (its own signal.Notify) or "return" (its own and ReturnInterrupt).
const runnerEnv = "PROCGROUP_TEST_RUNNER"

// runner runs a forking child under a *Cmd. Unless it dies of the signal
// it prints "sig=<returned signal> count=<signals its own handler saw>
// lag_ms=<from its handler's first signal to Wait's return>" and exits 5.
func runner() int {
	mode := os.Getenv(runnerEnv)
	ch := make(chan os.Signal, 4)
	first := make(chan time.Time, 1)
	if mode != "" {
		signal.Notify(ch, syscall.SIGINT)
	}
	self, err := os.Executable()
	if err != nil {
		fmt.Println(err)
		return 4
	}
	c := CommandContext(context.Background(), self)
	c.Env = append(os.Environ(), helperEnv+"=fork")
	c.ReturnInterrupt = mode == "return"
	count := make(chan int)
	go func() {
		n := 0
		for {
			select {
			case <-ch:
				if n == 0 {
					first <- time.Now()
				}
				n++
			case count <- n:
				return
			}
		}
	}()
	err = c.Run()
	end := time.Now()
	var in *InterruptError
	if !errors.As(err, &in) {
		fmt.Printf("Run = %v, want an *InterruptError\n", err)
		return 3
	}
	var lag time.Duration
	select {
	case t0 := <-first:
		lag = end.Sub(t0)
	case <-time.After(2 * time.Second):
		lag = -1
	}
	time.Sleep(300 * time.Millisecond) // a late duplicate still counts
	fmt.Printf("sig=%s count=%d lag_ms=%d\n", in.Signal, <-count, lag.Milliseconds())
	return 5
}

type runnerProc struct {
	cmd          *exec.Cmd
	out          *strings.Builder
	child, grand int
}

type runnerResult struct {
	sig   string
	count int
	lag   time.Duration
}

// startRunner re-runs the test binary as the runner and returns once its
// forking child has written the pids.
func startRunner(t *testing.T, mode string) *runnerProc {
	t.Helper()
	needProc(t)
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	pidFile := filepath.Join(t.TempDir(), "pids")
	r := &runnerProc{cmd: exec.Command(self), out: &strings.Builder{}}
	r.cmd.Env = append(os.Environ(), helperEnv+"=runner", runnerEnv+"="+mode, pidFileEnv+"="+pidFile)
	r.cmd.Stdout, r.cmd.Stderr = r.out, os.Stderr
	if err := r.cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if r.cmd.ProcessState == nil {
			_ = r.cmd.Process.Kill()
			_ = r.cmd.Wait()
		}
	})
	var ok bool
	if r.child, r.grand, ok = waitPids(deadline(t, 10*time.Second), pidFile); !ok {
		t.Fatal("the runner's child never wrote its pids")
	}
	return r
}

// interrupt sends SIGINT to a catching runner and parses its report.
func (r *runnerProc) interrupt(t *testing.T) runnerResult {
	t.Helper()
	if err := r.cmd.Process.Signal(syscall.SIGINT); err != nil {
		t.Fatal(err)
	}
	var ee *exec.ExitError
	if err := r.cmd.Wait(); !errors.As(err, &ee) || ee.ExitCode() != 5 {
		t.Fatalf("runner ended with %v, want exit 5\n%s", err, r.out)
	}
	var res runnerResult
	var lagMS int64
	if _, err := fmt.Sscanf(r.out.String(), "sig=%s count=%d lag_ms=%d", &res.sig, &res.count, &lagMS); err != nil {
		t.Fatalf("runner report %q: %v", r.out, err)
	}
	res.lag = time.Duration(lagMS) * time.Millisecond
	return res
}

// forking is a *Cmd of the forking helper and the pid file it writes.
func forking(t *testing.T, ctx context.Context) (*Cmd, string) {
	pidFile := filepath.Join(t.TempDir(), "pids")
	return helper(t, ctx, "fork", pidFileEnv+"="+pidFile), pidFile
}

// deadline is a context that ends after d or with the test.
func deadline(t *testing.T, d time.Duration) context.Context {
	ctx, cancel := context.WithTimeout(context.Background(), d)
	t.Cleanup(cancel)
	return ctx
}

// waitPids polls the pid file of a forking helper until ctx ends.
func waitPids(ctx context.Context, path string) (child, grand int, ok bool) {
	for {
		if raw, err := os.ReadFile(path); err == nil {
			if f := strings.Fields(string(raw)); len(f) == 2 {
				c, err1 := strconv.Atoi(f[0])
				g, err2 := strconv.Atoi(f[1])
				if err1 == nil && err2 == nil {
					return c, g, true
				}
			}
		}
		select {
		case <-ctx.Done():
			return 0, 0, false
		case <-time.After(20 * time.Millisecond):
		}
	}
}

// needProc skips a test that tells a zombie from a running process
// through /proc.
func needProc(t *testing.T) {
	t.Helper()
	if _, err := os.Stat("/proc/self/stat"); err != nil {
		t.Skip("no /proc to tell a zombie from a running process")
	}
}

// waitGone fails the test unless pid is gone (or a zombie) within 3 s; it
// kills a survivor.
func waitGone(t *testing.T, pid int) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for alive(pid) {
		if time.Now().After(deadline) {
			_ = syscall.Kill(pid, syscall.SIGKILL)
			t.Fatalf("process %d still running", pid)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// alive reports whether pid runs and is not a zombie waiting to be reaped.
func alive(pid int) bool {
	if syscall.Kill(pid, 0) != nil {
		return false
	}
	stat, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/stat")
	if err != nil {
		return true
	}
	fields := strings.Fields(string(stat))
	return len(fields) < 3 || fields[2] != "Z"
}
