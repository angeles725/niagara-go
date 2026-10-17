package pycompat

import "syscall"

const (
	errorInsufficientBuffer = 122    // ERROR_INSUFFICIENT_BUFFER
	langNeutralDefault      = 0x0400 // MAKELANGID(LANG_NEUTRAL, SUBLANG_DEFAULT)
)

// systemMessage is FormatMessageW with CPython's flags and language. CPython lets the system
// allocate the buffer; a caller buffer is grown instead until the message fits.
func systemMessage(code uint32) (string, bool) {
	const flags = syscall.FORMAT_MESSAGE_FROM_SYSTEM | syscall.FORMAT_MESSAGE_IGNORE_INSERTS
	for size := 512; size <= 1<<16; size *= 2 {
		buf := make([]uint16, size)
		//lint:ignore SA1019 the syscall wrapper avoids a direct golang.org/x/sys dependency
		n, err := syscall.FormatMessage(flags, 0, code, langNeutralDefault, buf, nil)
		if err == nil && n > 0 {
			return syscall.UTF16ToString(buf[:n]), true
		}
		if err != syscall.Errno(errorInsufficientBuffer) {
			return "", false
		}
	}
	return "", false
}
