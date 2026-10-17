//go:build linux

package argparse

import (
	"syscall"
	"unsafe"
)

// stdoutColumns is os.get_terminal_size(1).columns, 0 when stdout is not a terminal.
func stdoutColumns() int {
	var ws struct{ row, col, xpixel, ypixel uint16 }
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, 1, syscall.TIOCGWINSZ, uintptr(unsafe.Pointer(&ws)))
	if errno != 0 {
		return 0
	}
	return int(ws.col)
}
