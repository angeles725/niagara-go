//go:build !unix

package helpbuild

import (
	"context"
	"os"
	"os/exec"
)

// killGroupOnCancel keeps the default kill of the direct child, which is
// what Python's subprocess.run does on a timeout.
func killGroupOnCancel(cmd *exec.Cmd) {}

// relayInterrupts does nothing: without a separate process group, the
// console's Ctrl-C reaches pdftotext directly.
func relayInterrupts(context.CancelCauseFunc) (stop func()) { return func() {} }

// reraise is never reached: relayInterrupts never cancels.
func reraise(os.Signal) {}
