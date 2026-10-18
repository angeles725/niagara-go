//go:build unix

package procgroup

import (
	"context"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
)

// inGroup makes cmd start in its own process group, keeping an existing
// SysProcAttr, and its cancel kill the whole group.
func inGroup(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.Setpgid = true
	cmd.Cancel = func() error { return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL) }
}

// relayInterrupts makes SIGINT and SIGTERM, unless ignored at start, cancel
// the run with an *InterruptError cause. The returned stop ends the relay
// and reports the signal that came, nil for none; it restores the default
// handling of both signals while no other signal.Notify wants them.
func relayInterrupts(cancel context.CancelCauseFunc) (stop func() os.Signal) {
	var sigs []os.Signal
	for _, s := range []os.Signal{syscall.SIGINT, syscall.SIGTERM} {
		if !signal.Ignored(s) {
			sigs = append(sigs, s)
		}
	}
	if len(sigs) == 0 {
		return func() os.Signal { return nil }
	}
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, sigs...)
	quit, done := make(chan struct{}), make(chan struct{})
	var got os.Signal
	go func() {
		defer close(done)
		select {
		case got = <-ch:
		case <-quit:
			select { // a signal that came between the child's end and stop
			case got = <-ch:
			default:
			}
		}
		if got != nil {
			cancel(&InterruptError{got})
		}
	}()
	return func() os.Signal {
		signal.Stop(ch)
		close(quit)
		<-done
		return got
	}
}
