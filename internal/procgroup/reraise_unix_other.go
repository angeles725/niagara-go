//go:build unix && !linux

package procgroup

import (
	"os"
	"syscall"
	"time"
)

// reraiseWait bounds how long reraise lets the runtime deliver the signal
// to some thread of this process where it cannot be sent to the calling
// thread alone.
const reraiseWait = 250 * time.Millisecond

// reraise sends sig, whose default handling stop restored, to this
// process. The signal may land on another thread, so reraise waits up to
// reraiseWait for it to kill the process; it returns after that when
// another signal.Notify caught it.
func reraise(sig os.Signal) {
	if s, ok := sig.(syscall.Signal); ok && syscall.Kill(os.Getpid(), s) == nil {
		time.Sleep(reraiseWait)
	}
}
