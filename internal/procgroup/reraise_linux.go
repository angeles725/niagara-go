package procgroup

import (
	"os"
	"runtime"
	"syscall"
)

// reraise sends sig, whose default handling stop restored, to the calling
// thread. Linux delivers a thread-directed signal that is not blocked
// before the tgkill system call returns, so either the runtime's default
// handling kills the process right there, or another signal.Notify caught
// the signal and reraise returns at once; there is nothing to wait for.
func reraise(sig os.Signal) {
	s, ok := sig.(syscall.Signal)
	if !ok {
		return
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	_ = syscall.Tgkill(os.Getpid(), syscall.Gettid(), s)
}
