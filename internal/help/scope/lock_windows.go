//go:build windows

package scope

import (
	"errors"
	"syscall"
)

const (
	processQueryLimitedInformation = 0x1000 // PROCESS_QUERY_LIMITED_INFORMATION
	stillActive                    = 259    // STILL_ACTIVE exit code
	errorInvalidParameter          = syscall.Errno(87)
)

// pidAlive reports whether process pid is running on this host: OpenProcess
// fails with ERROR_INVALID_PARAMETER for a pid no process has, and an exited
// process that is still referenced has an exit code other than STILL_ACTIVE.
// Access denied means it exists. known is false when the answer is unclear,
// and the lock is then treated as live.
func pidAlive(pid int) (alive, known bool) {
	if pid <= 0 {
		return false, false
	}
	h, err := syscall.OpenProcess(processQueryLimitedInformation, false, uint32(pid))
	if err != nil {
		switch {
		case errors.Is(err, errorInvalidParameter):
			return false, true
		case errors.Is(err, syscall.ERROR_ACCESS_DENIED):
			return true, true
		}
		return false, false
	}
	defer syscall.CloseHandle(h)
	var code uint32
	if err := syscall.GetExitCodeProcess(h, &code); err != nil {
		return false, false
	}
	return code == stillActive, true
}
