//go:build !unix

package procgroup

import (
	"context"
	"os"
	"os/exec"
)

// inGroup keeps exec's default cancel, which kills the direct child.
func inGroup(*exec.Cmd) {}

// relayInterrupts relays nothing: without a separate process group, the
// console's Ctrl-C reaches the child directly.
func relayInterrupts(context.CancelCauseFunc) (stop func() os.Signal) {
	return func() os.Signal { return nil }
}

// reraise is never reached: relayInterrupts never reports a signal.
func reraise(os.Signal) {}
