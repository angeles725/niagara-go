//go:build unix

package searchdoor

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// forkingTool is a backend that starts a grandchild on its own stdout, writes
// the grandchild's pid to the file in argv[1], prints "row1" and then either
// exits 0 (parentExits) or sleeps past any deadline. The grandchild sleeps
// grandSleep seconds, then prints "row2".
func forkingTool(t *testing.T, grandSleep float64, parentExits bool) string {
	t.Helper()
	tail := "time.sleep(30)"
	if parentExits {
		tail = "sys.exit(0)"
	}
	return script(t, `import subprocess, sys, time
g = subprocess.Popen([sys.executable, "-c",
    "import time; time.sleep(`+strconv.FormatFloat(grandSleep, 'f', -1, 64)+`); print('row2', flush=True)"])
with open(sys.argv[1], "w") as f:
    f.write(str(g.pid))
print("row1", flush=True)
`+tail+"\n")
}

// grandchildPid reads the pid the forking tool wrote.
func grandchildPid(t *testing.T, pidFile string) int {
	t.Helper()
	b, err := os.ReadFile(pidFile)
	if err != nil {
		t.Fatalf("the tool never wrote its grandchild's pid: %v", err)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(b)))
	if err != nil {
		t.Fatal(err)
	}
	return pid
}

// running reports whether pid runs and is not a zombie waiting to be reaped.
func running(pid int) bool {
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

// requireGone fails unless pid is gone (or a zombie) within 2 s; it kills a
// survivor so the test leaves nothing behind.
func requireGone(t *testing.T, pid int) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for running(pid) {
		if time.Now().After(deadline) {
			_ = syscall.Kill(pid, syscall.SIGKILL)
			t.Fatalf("grandchild %d outlived the backend's run", pid)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// A backend's timeout kills everything it started, and a grandchild that
// holds the output pipe after its parent exited is waited for up to the
// deadline, as Python's subprocess.run(capture_output=True, timeout=...)
// reads until EOF or the timeout.
func TestExecBackendProcessGroup(t *testing.T) {
	if testing.Short() {
		t.Skip("waits for backend timeouts")
	}
	if _, err := os.Stat("/proc/self/stat"); err != nil {
		t.Skip("no /proc to tell a zombie from a running process")
	}
	py := python(t)
	b := execBackend{python: func() (string, error) { return py, nil }}
	timedOut := func(t *testing.T, got Raw, timeout string) {
		t.Helper()
		if got.State != Unavailable || got.Reason != "unavailable: timed out after "+timeout+"s" || got.HasRC {
			t.Errorf("raw = %+v, want the timeout", got)
		}
	}
	t.Run("a timeout kills the grandchild too", func(t *testing.T) {
		pidFile := filepath.Join(t.TempDir(), "pid")
		got := b.Run([]string{forkingTool(t, 30, false), pidFile}, 1)
		timedOut(t, got, "1")
		requireGone(t, grandchildPid(t, pidFile))
	})
	t.Run("a grandchild that ends before the deadline is read to the end", func(t *testing.T) {
		pidFile := filepath.Join(t.TempDir(), "pid")
		got := b.Run([]string{forkingTool(t, 0.5, true), pidFile}, 10)
		if got.State != OK || got.Out != "row1\nrow2" || got.RC != 0 {
			t.Errorf("raw = %+v, want OK with both rows", got)
		}
		requireGone(t, grandchildPid(t, pidFile))
	})
	t.Run("a grandchild holding the pipe past the deadline is a timeout and is killed", func(t *testing.T) {
		pidFile := filepath.Join(t.TempDir(), "pid")
		t0 := time.Now()
		got := b.Run([]string{forkingTool(t, 30, true), pidFile}, 1)
		timedOut(t, got, "1")
		if d := time.Since(t0); d > 5*time.Second {
			t.Errorf("Run took %v for a 1 s timeout", d)
		}
		requireGone(t, grandchildPid(t, pidFile))
	})
}
