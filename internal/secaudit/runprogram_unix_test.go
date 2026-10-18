//go:build unix

package secaudit

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/angeles725/niagara-go/internal/cli"
)

// forkingKeytool is a keytool wrapper that forks its "JVM" (a long sleep), writes the fork's pid
// to child.pid next to the script and waits for it, like a wrapper that does not exec java.
func forkingKeytool(sleep, dir string) string {
	return sleep + " 30 &\necho $! > " + filepath.Join(dir, "child.pid") + "\nwait\n"
}

// A keytool wrapper that forks its JVM instead of exec'ing it must not leave the JVM running
// after the timeout: runProgram runs it through internal/procgroup, which kills the whole process
// group (the signal relay and its tests live there).
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

// interruptHelperEnv makes the test binary act as the audit process of killsChildGroupOn: its
// value is the keytool timeout (fakeKeytool sets it in the parent only) and interruptHomeEnv the
// Niagara home to audit.
const (
	interruptHelperEnv = "SECAUDIT_INTERRUPT_HELPER_TIMEOUT"
	interruptHomeEnv   = "SECAUDIT_INTERRUPT_HELPER_HOME"
)

// Ctrl-C on the audit while keytool runs kills keytool's whole process group, which does not get
// the terminal's SIGINT itself, and the audit dies of SIGINT (130) without printing a report.
func TestInterruptKillsChildGroup(t *testing.T) { killsChildGroupOn(t, syscall.SIGINT) }

// SIGTERM to the audit does the same, and the audit dies of SIGTERM (143).
func TestTerminateKillsChildGroup(t *testing.T) { killsChildGroupOn(t, syscall.SIGTERM) }

// killsChildGroupOn re-runs the test binary as the audit (Main on a home with a truststore, so
// the audit runs the forking fake keytool), waits for the fake keytool to fork, signals the audit
// and checks that it died of sig, printed nothing on stdout and that the forked child is gone.
func killsChildGroupOn(t *testing.T, sig syscall.Signal) {
	if v := os.Getenv(interruptHelperEnv); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			fmt.Printf("bad %s=%q: %v\n", interruptHelperEnv, v, err)
			os.Exit(4)
		}
		keytoolTimeout = d
		code := Main([]string{os.Getenv(interruptHomeEnv)}, cli.Stdio{In: os.Stdin, Out: os.Stdout, Err: os.Stderr})
		fmt.Printf("audit survived %s: exit %d\n", sig, code)
		os.Exit(3)
	}
	needProc(t)
	dir := fakeKeytool(t, 30*time.Second, forkingKeytool)
	home := homeWithTruststore(t)
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(self, "-test.run=^"+t.Name()+"$")
	cmd.Env = append(os.Environ(), interruptHelperEnv+"="+keytoolTimeout.String(), interruptHomeEnv+"="+home)
	var out bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, os.Stderr
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	waited := false
	defer func() {
		if !waited {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
		}
	}()
	pidFile := filepath.Join(dir, "child.pid")
	deadline := time.Now().Add(10 * time.Second)
	pid, ok := readPid(pidFile)
	for ; !ok; pid, ok = readPid(pidFile) {
		if time.Now().After(deadline) {
			t.Fatal("the fake keytool never forked its child")
		}
		time.Sleep(20 * time.Millisecond)
	}
	if err := cmd.Process.Signal(sig); err != nil {
		t.Fatal(err)
	}
	err = cmd.Wait()
	waited = true
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		t.Errorf("audit process ended with %v, want death by %s", err, sig)
	} else if ws := exitErr.Sys().(syscall.WaitStatus); !ws.Signaled() || ws.Signal() != sig {
		t.Errorf("audit process ended with %v, want death by %s", exitErr, sig)
	}
	if out.Len() > 0 {
		t.Errorf("audit printed on stdout, want no report:\n%s", out.String())
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
