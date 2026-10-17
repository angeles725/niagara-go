package pycompat

import (
	"fmt"
	"strings"
)

// WinErrorText is the strerror of the OSError CPython raises for the Win32 or Winsock error
// code (PyErr_SetExcFromWindowsErr in Python/errors.c, used by the socket module and the os
// functions that report "[WinError n]"): the system message from
// FormatMessageW(FROM_SYSTEM|IGNORE_INSERTS) in MAKELANGID(LANG_NEUTRAL, SUBLANG_DEFAULT),
// so in the language of the Windows installation, with trailing whitespace, control
// characters and dots removed; "Windows Error 0x<hex>" when the system has no message for
// the code. Go's syscall.Errno.Error asks for English first and keeps the dot, so it differs
// from Python on a non-English Windows.
//
// Off Windows the text comes from a built-in table of Windows' English messages for common
// file and socket codes (winerror_other.go); Python never prints "[WinError n]" there, so
// the table only keeps the result deterministic.
func WinErrorText(code uint32) string {
	if msg, ok := systemMessage(code); ok {
		return trimWinMessage(msg)
	}
	return fmt.Sprintf("Windows Error 0x%x", code)
}

// WinErrorString is str() of that OSError without a filename: "[WinError n] <WinErrorText>".
// CPython keeps the code in a C int, so codes above 0x7fffffff print negative.
func WinErrorString(code uint32) string {
	return fmt.Sprintf("[WinError %d] %s", int32(code), WinErrorText(code))
}

// trimWinMessage drops what PyErr_SetExcFromWindowsErr strips from the end of a message:
// every character <= ' ' (CR, LF, spaces, other controls) and '.'.
func trimWinMessage(s string) string {
	return strings.TrimRightFunc(s, func(r rune) bool { return r <= ' ' || r == '.' })
}
