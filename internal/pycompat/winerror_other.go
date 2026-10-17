//go:build !windows

package pycompat

// winEnglish holds Windows' English FormatMessage texts (as syscall.Errno.Error prints them
// on Windows) for common file and socket codes. It is used only off Windows, where Python
// never reports a WinError, to keep WinErrorText deterministic.
var winEnglish = map[uint32]string{
	1:     "Incorrect function.",
	2:     "The system cannot find the file specified.",
	3:     "The system cannot find the path specified.",
	5:     "Access is denied.",
	32:    "The process cannot access the file because it is being used by another process.",
	87:    "The parameter is incorrect.",
	10013: "An attempt was made to access a socket in a way forbidden by its access permissions.",
	10048: "Only one usage of each socket address (protocol/network address/port) is normally permitted.",
	10049: "The requested address is not valid in its context.",
	10050: "A socket operation encountered a dead network.",
	10051: "A socket operation was attempted to an unreachable network.",
	10054: "An existing connection was forcibly closed by the remote host.",
	10060: "A connection attempt failed because the connected party did not properly respond after a period of time, or established connection failed because connected host has failed to respond.",
	10061: "No connection could be made because the target machine actively refused it.",
	10065: "A socket operation was attempted to an unreachable host.",
	11001: "No such host is known.",
	11004: "The requested name is valid, but no data of the requested type was found.",
}

func systemMessage(code uint32) (string, bool) {
	s, ok := winEnglish[code]
	return s, ok
}
