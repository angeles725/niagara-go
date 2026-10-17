//go:build unix

package secaudit

import (
	"context"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"
)

// killTreeOnCancel starts a probe child (keytool, openssl, icacls) in its own process group and
// makes a cancel (timeout or interrupt) kill the whole group, so a wrapper script that forks java
// instead of exec'ing it leaves nothing running. An existing SysProcAttr is kept.
func killTreeOnCancel(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.Setpgid = true
	cmd.Cancel = func() error { return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL) }
}

// relayInterrupts covers the time a probe child runs: its own process group does not get the
// terminal's Ctrl-C, so SIGINT or SIGTERM to the audit cancels the run with an *interruptError
// cause instead, and the cancel kills the child group. The returned stop must be called once the
// child has been waited for; it restores the default handling of both signals. Signals the audit
// was started with ignored stay ignored.
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

// reraise sends sig, now back to its default handling, to the audit itself so it dies of the
// signal as it would have without the relay (the shell sees 130 or 143). The wait only lets the
// runtime deliver it; the caller reports the interrupt if the process somehow lives on.
func reraise(sig os.Signal) {
	if s, ok := sig.(syscall.Signal); ok && syscall.Kill(os.Getpid(), s) == nil {
		time.Sleep(time.Second)
	}
}
