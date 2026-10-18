//go:build !windows

package scope

import (
	"errors"
	"syscall"
)

// pidAlive reports whether process pid is running on this host: signal 0
// checks for it without disturbing it (ESRCH: no such process; EPERM: it
// exists but belongs to another user). known is false when the answer is
// unclear, and the lock is then treated as live.
func pidAlive(pid int) (alive, known bool) {
	if pid <= 0 {
		return false, false
	}
	switch err := syscall.Kill(pid, 0); {
	case err == nil, errors.Is(err, syscall.EPERM):
		return true, true
	case errors.Is(err, syscall.ESRCH):
		return false, true
	}
	return false, false
}
