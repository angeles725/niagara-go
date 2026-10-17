//go:build !unix

package secaudit

import (
	"context"
	"os"
	"os/exec"
)

// killTreeOnCancel keeps the default kill of the direct child: keytool.exe runs the JVM in-process.
func killTreeOnCancel(cmd *exec.Cmd) {}

// relayInterrupts does nothing: without a separate process group, the console's Ctrl-C reaches
// every probe child (keytool, openssl, icacls) directly.
func relayInterrupts(context.CancelCauseFunc) (stop func()) { return func() {} }

// reraise is never reached: relayInterrupts never cancels.
func reraise(os.Signal) {}
