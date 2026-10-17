//go:build !windows

package bbmdverify

import "syscall"

const errnoLabel = "Errno"

// setSockOpts mirrors make_socket: SO_BROADCAST, then SO_REUSEADDR (best effort).
func setSockOpts(fd uintptr) error {
	if err := syscall.SetsockoptInt(int(fd), syscall.SOL_SOCKET, syscall.SO_BROADCAST, 1); err != nil {
		return err
	}
	_ = syscall.SetsockoptInt(int(fd), syscall.SOL_SOCKET, syscall.SO_REUSEADDR, 1)
	return nil
}
