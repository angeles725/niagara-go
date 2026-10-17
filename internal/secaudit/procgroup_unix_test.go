//go:build unix

package secaudit

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// forkingKeytool is a keytool wrapper that forks its "JVM" (a long sleep), writes the fork's pid
// to child.pid next to the script and waits for it, like a wrapper that does not exec java.
func forkingKeytool(sleep, dir string) string {
	return sleep + " 30 &\necho $! > " + filepath.Join(dir, "child.pid") + "\nwait\n"
}

// A keytool wrapper that forks its JVM instead of exec'ing it must not leave the JVM running
// after the timeout: the whole process group is killed.
func TestTimeoutKillsForkedChildren(t *testing.T) {
	needProc(t)
	dir := fakeKeytool(t, time.Second, forkingKeytool)
	if o, err := defaultEnv().keytoolList("ts.jks", false); err != nil || o.failure != "timed out after 1 s" {
		t.Fatalf("keytoolList = %+v, %v", o, err)
	}
	pid, ok := readPid(filepath.Join(dir, "child.pid"))
	if !ok {
		t.Skip("the fake keytool did not fork its child within the 1 s timeout")
	}
	waitGone(t, pid)
}

// needProc skips a test that tells a zombie from a running process through /proc.
func needProc(t *testing.T) {
	t.Helper()
	if _, err := os.Stat("/proc/self/stat"); err != nil {
		t.Skip("no /proc to tell a zombie from a running process")
	}
}

// readPid reads a pid file; ok is false while the file is missing or not yet complete.
func readPid(path string) (pid int, ok bool) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return 0, false
	}
	pid, err = strconv.Atoi(strings.TrimSpace(string(raw)))
	return pid, err == nil
}

// waitGone fails the test unless pid is gone (or a zombie) within 3 s; it kills a survivor.
func waitGone(t *testing.T, pid int) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for alive(pid) {
		if time.Now().After(deadline) {
			_ = syscall.Kill(pid, syscall.SIGKILL)
			t.Fatalf("forked child %d still running", pid)
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
