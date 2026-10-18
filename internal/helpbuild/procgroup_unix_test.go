//go:build unix

package helpbuild

import (
	"errors"
	"fmt"
	"io/fs"
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

// A fake pdftotext that forks its real work instead of exec'ing it leaves
// nothing running after the timeout: the whole process group is killed.
func TestPDFTimeoutKillsForkedChildren(t *testing.T) {
	needProc(t)
	script, dir := fakePdftotext(t)
	pdf := filepath.Join(dir, "x.pdf")
	writeFile(t, pdf+".fork", nil)
	_, _, err := ExtractPDFText(script, pdf, time.Second)
	var pe *PDFError
	if !errors.As(err, &pe) || pe.Failure != PDFTimedOut {
		t.Fatalf("ExtractPDFText = %v, want a timeout", err)
	}
	pid, ok := readPid(pdf + ".pid")
	if !ok {
		t.Skip("the fake pdftotext did not fork within the 1 s timeout")
	}
	waitGone(t, pid)
}

// pdfInterruptHelperEnv makes the test binary act as the extractor process
// of killsPdftotextGroupOn: "<script>\n<pdf>".
const pdfInterruptHelperEnv = "HELPBUILD_INTERRUPT_HELPER"

// Ctrl-C on the extractor while pdftotext runs kills pdftotext's whole
// process group, which does not get the terminal's SIGINT itself, and the
// extractor then dies of SIGINT as Python does on KeyboardInterrupt (130).
func TestPDFInterruptKillsChildGroup(t *testing.T) { killsPdftotextGroupOn(t, syscall.SIGINT) }

// SIGTERM to the extractor does the same, and it dies of SIGTERM (143);
// Python dies of it too but leaves pdftotext running.
func TestPDFTerminateKillsChildGroup(t *testing.T) { killsPdftotextGroupOn(t, syscall.SIGTERM) }

// A caller that catches SIGINT itself (signal.Notify) keeps the process
// alive: ExtractPDFs then returns the interrupt, as Python raises
// KeyboardInterrupt, instead of counting a failed PDF, and saves no index.
func TestPDFInterruptCaughtStopsTheRun(t *testing.T) {
	if v := os.Getenv(pdfInterruptHelperEnv); v != "" {
		script, pdf, _ := strings.Cut(v, "\n")
		signal.Notify(make(chan os.Signal, 1), syscall.SIGINT)
		root := filepath.Join(filepath.Dir(pdf), "root")
		res, err := ExtractPDFs(root, PDFOptions{DocsDir: filepath.Dir(pdf), Pdftotext: script, Timeout: 30 * time.Second})
		var in *interruptError
		_, statErr := os.Stat(filepath.Join(root, "indexes", "pdf-index.json"))
		if errors.As(err, &in) && in.sig == syscall.SIGINT && errors.Is(statErr, fs.ErrNotExist) && res.Stats.Failed == 0 {
			os.Exit(5)
		}
		fmt.Printf("ExtractPDFs = %+v, %v; index: %v\n", res.Stats, err, statErr)
		os.Exit(3)
	}
	cmd, pid := startInterruptHelper(t)
	if err := cmd.Process.Signal(syscall.SIGINT); err != nil {
		t.Fatal(err)
	}
	var exitErr *exec.ExitError
	if err := cmd.Wait(); !errors.As(err, &exitErr) || exitErr.ExitCode() != 5 {
		t.Errorf("extractor ended with %v, want exit 5 (the interrupt returned)", err)
	}
	waitGone(t, pid)
}

// killsPdftotextGroupOn re-runs the test binary as the extractor so sig
// hits a separate process, waits for the fake pdftotext to fork, signals
// the extractor and checks that it died of sig and the fork is gone.
func killsPdftotextGroupOn(t *testing.T, sig syscall.Signal) {
	if v := os.Getenv(pdfInterruptHelperEnv); v != "" {
		script, pdf, _ := strings.Cut(v, "\n")
		_, _, err := ExtractPDFText(script, pdf, 30*time.Second)
		fmt.Printf("ExtractPDFText survived %s: %v\n", sig, err)
		os.Exit(3)
	}
	cmd, pid := startInterruptHelper(t)
	if err := cmd.Process.Signal(sig); err != nil {
		t.Fatal(err)
	}
	err := cmd.Wait()
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		t.Errorf("extractor ended with %v, want death by %s", err, sig)
	} else if ws := exitErr.Sys().(syscall.WaitStatus); !ws.Signaled() || ws.Signal() != sig {
		t.Errorf("extractor ended with %v, want death by %s", exitErr, sig)
	}
	waitGone(t, pid)
}

// startInterruptHelper re-runs the running test as the extractor over a
// fake pdftotext that forks, and returns once the fork's pid is known; the
// caller signals and waits for the helper (a cleanup kills it otherwise).
func startInterruptHelper(t *testing.T) (*exec.Cmd, int) {
	t.Helper()
	needProc(t)
	script, dir := fakePdftotext(t)
	pdf := filepath.Join(dir, "docs", "x.pdf")
	writeFile(t, pdf, []byte("%PDF"))
	writeFile(t, pdf+".fork", nil)
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(self, "-test.run=^"+t.Name()+"$")
	cmd.Env = append(os.Environ(), pdfInterruptHelperEnv+"="+script+"\n"+pdf)
	cmd.Stdout, cmd.Stderr = os.Stderr, os.Stderr
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if cmd.ProcessState == nil {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
		}
	})
	deadline := time.Now().Add(10 * time.Second)
	pid, ok := readPid(pdf + ".pid")
	for ; !ok; pid, ok = readPid(pdf + ".pid") {
		if time.Now().After(deadline) {
			t.Fatal("the fake pdftotext never forked its child")
		}
		time.Sleep(20 * time.Millisecond)
	}
	return cmd, pid
}

// needProc skips a test that tells a zombie from a running process
// through /proc.
func needProc(t *testing.T) {
	t.Helper()
	if _, err := os.Stat("/proc/self/stat"); err != nil {
		t.Skip("no /proc to tell a zombie from a running process")
	}
}

// readPid reads a pid file; ok is false while it is missing or incomplete.
func readPid(path string) (pid int, ok bool) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return 0, false
	}
	pid, err = strconv.Atoi(strings.TrimSpace(string(raw)))
	return pid, err == nil
}

// waitGone fails the test unless pid is gone (or a zombie) within 3 s; it
// kills a survivor.
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
