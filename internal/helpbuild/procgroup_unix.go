//go:build unix

package helpbuild

import (
	"context"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"
)

// Copied from internal/secaudit/procgroup_unix.go (relayInterrupts,
// reraise); see docs/helpbuild.md for the shared-package proposal.

// killGroupOnCancel starts a child (pdftotext) in its own process group and
// makes a cancel (the timeout or an interrupt) kill the whole group, so a
// wrapper script that forks the real program leaves nothing running.
// Python's subprocess.run kills only the direct child.
func killGroupOnCancel(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error { return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL) }
}

// relayInterrupts covers the time a pdftotext child runs: its own process
// group does not get the terminal's Ctrl-C, so SIGINT or SIGTERM to this
// process cancels the run with an *interruptError cause instead, and the
// cancel kills the child group. The returned stop must be called once the
// child has been waited for; it restores the default handling of both
// signals only while no other signal.Notify for them is active in the
// process. Signals the process was started with ignored stay ignored.
func relayInterrupts(cancel context.CancelCauseFunc) (stop func()) {
	var sigs []os.Signal
	for _, s := range []os.Signal{syscall.SIGINT, syscall.SIGTERM} {
		if !signal.Ignored(s) {
			sigs = append(sigs, s)
		}
	}
	if len(sigs) == 0 {
		return func() {}
	}
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, sigs...)
	quit, done := make(chan struct{}), make(chan struct{})
	go func() {
		defer close(done)
		select {
		case s := <-ch:
			cancel(&interruptError{s})
		case <-quit:
			select { // a signal that came between the child's end and stop
			case s := <-ch:
				cancel(&interruptError{s})
			default:
			}
		}
	}()
	return func() {
		signal.Stop(ch)
		close(quit)
		<-done
	}
}

// reraise sends sig, back to its default handling once stop ran, to this
// process so it dies of the signal as it would have without the relay (the
// shell sees 130 or 143), as Python does on an uncaught KeyboardInterrupt.
// The wait only lets the runtime deliver it; the caller returns the
// interrupt if the process lives on (another signal.Notify catches sig).
func reraise(sig os.Signal) {
	if s, ok := sig.(syscall.Signal); ok && syscall.Kill(os.Getpid(), s) == nil {
		time.Sleep(time.Second)
	}
}
