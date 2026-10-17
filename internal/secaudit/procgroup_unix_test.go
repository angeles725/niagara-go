//go:build unix

package secaudit

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// A keytool wrapper that forks its JVM instead of exec'ing it must not leave the JVM running
// after the timeout: the whole process group is killed.
func TestTimeoutKillsForkedChildren(t *testing.T) {
	sleep, err := exec.LookPath("sleep")
	if err != nil {
		t.Skip("no sleep on PATH")
	}
	bin := t.TempDir()
	pidFile := filepath.Join(bin, "child.pid")
	script := "#!/bin/sh\n" + sleep + " 30 &\necho $! > " + pidFile + "\nwait\n"
	if err := os.WriteFile(filepath.Join(bin, "keytool"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)
	saved := keytoolTimeout
	keytoolTimeout = 200 * time.Millisecond
	defer func() { keytoolTimeout = saved }()
	if o, err := defaultEnv().keytoolList("ts.jks", false); err != nil || o.failure != "timed out after 0.2 s" {
		t.Fatalf("keytoolList = %+v, %v", o, err)
	}
	raw, err := os.ReadFile(pidFile)
	if err != nil {
		t.Fatal(err)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(raw)))
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(3 * time.Second)
	for alive(pid) {
		if time.Now().After(deadline) {
			_ = syscall.Kill(pid, syscall.SIGKILL)
			t.Fatalf("forked child %d still running after the timeout", pid)
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
