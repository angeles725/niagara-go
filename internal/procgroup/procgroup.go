package procgroup

import (
	"context"
	"errors"
	"os"
	"os/exec"
)

// InterruptError is a SIGINT or SIGTERM that came while the child ran; the
// child's process group was killed for it.
type InterruptError struct{ Signal os.Signal }

func (e *InterruptError) Error() string { return "interrupted by " + e.Signal.String() }

// IsInterrupt reports whether err is, or wraps, the *InterruptError of a
// SIGINT or SIGTERM that came while a child ran. A caller returns such an
// error up instead of reading it as a failed run.
func IsInterrupt(err error) bool {
	var in *InterruptError
	return errors.As(err, &in)
}

// Cmd is an *exec.Cmd run in its own process group (see the package doc).
// Configure the embedded fields (Stdin, Stdout, Env, WaitDelay, the pipe
// methods...) as for exec.Cmd before Start.
type Cmd struct {
	*exec.Cmd

	// ReturnInterrupt makes Wait return the *InterruptError without
	// re-raising the signal on this process. A caller that registered its
	// own signal.Notify for SIGINT or SIGTERM sets it, since that handler
	// already got the signal and would get the re-raised one too.
	ReturnInterrupt bool

	cancel context.CancelCauseFunc
	stop   func() os.Signal // the relay of the current run; nil when none
}

// CommandContext is exec.CommandContext with the child in its own process
// group: ctx's cancel kills the whole group, and so does a relayed SIGINT
// or SIGTERM.
func CommandContext(ctx context.Context, name string, arg ...string) *Cmd {
	ctx, cancel := context.WithCancelCause(ctx)
	return &Cmd{Cmd: exec.CommandContext(ctx, name, arg...), cancel: cancel}
}

// Start starts the child in its own process group and relays SIGINT and
// SIGTERM until Wait. A start that fails releases both, as Wait would.
func (c *Cmd) Start() error {
	inGroup(c.Cmd)
	c.stop = relayInterrupts(c.cancel)
	if err := c.Cmd.Start(); err != nil {
		return c.finish(err)
	}
	return nil
}

// Wait waits for the child as exec.Cmd.Wait does, then ends the relay. If
// a SIGINT or SIGTERM came meanwhile, it re-raises it on this process
// (unless ReturnInterrupt) and returns an *InterruptError, whatever the
// child's own outcome.
func (c *Cmd) Wait() error { return c.finish(c.Cmd.Wait()) }

// Run starts the child and waits for it.
func (c *Cmd) Run() error {
	if err := c.Start(); err != nil {
		return err
	}
	return c.Wait()
}

// finish ends the relay of the run and releases the context, then turns a
// relayed signal into the run's outcome.
func (c *Cmd) finish(err error) error {
	var sig os.Signal
	if c.stop != nil {
		sig = c.stop()
		c.stop = nil
	}
	c.cancel(nil)
	if sig == nil {
		return err
	}
	if !c.ReturnInterrupt {
		reraise(sig)
	}
	return &InterruptError{sig}
}
