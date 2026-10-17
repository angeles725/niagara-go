package pycompat

import (
	"runtime"
	"strings"
	"testing"
)

func TestTrimWinMessage(t *testing.T) {
	for _, c := range [][2]string{
		{"Acceso denegado.\r\n", "Acceso denegado"},
		{"Solo se permite un uso de cada dirección de socket (protocolo/dirección de red/puerto)\r\n", "Solo se permite un uso de cada dirección de socket (protocolo/dirección de red/puerto)"},
		{"a. b...\t \x01", "a. b"},
		{"version 1.2", "version 1.2"},
		{" .\r\n", ""},
		{"", ""},
	} {
		if got := trimWinMessage(c[0]); got != c[1] {
			t.Errorf("trimWinMessage(%q) = %q, want %q", c[0], got, c[1])
		}
	}
}

// Codes no Windows release has a system message for format the same on every platform.
func TestWinErrorStringUnknown(t *testing.T) {
	for code, want := range map[uint32]string{
		0x2FFF:     "[WinError 12287] Windows Error 0x2fff",
		0xFFFF:     "[WinError 65535] Windows Error 0xffff",
		0x7FFFFFFF: "[WinError 2147483647] Windows Error 0x7fffffff",
		0xFFFFFFFF: "[WinError -1] Windows Error 0xffffffff",
	} {
		if got := WinErrorString(code); got != want {
			t.Errorf("WinErrorString(%#x) = %q, want %q", code, got, want)
		}
	}
}

func TestWinErrorEnglishFallback(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the English table is used only off Windows; see TestWinErrorStringMatchesPython")
	}
	for code, want := range map[uint32]string{
		1:     "[WinError 1] Incorrect function",
		2:     "[WinError 2] The system cannot find the file specified",
		3:     "[WinError 3] The system cannot find the path specified",
		5:     "[WinError 5] Access is denied",
		32:    "[WinError 32] The process cannot access the file because it is being used by another process",
		87:    "[WinError 87] The parameter is incorrect",
		10013: "[WinError 10013] An attempt was made to access a socket in a way forbidden by its access permissions",
		10048: "[WinError 10048] Only one usage of each socket address (protocol/network address/port) is normally permitted",
		10049: "[WinError 10049] The requested address is not valid in its context",
		10050: "[WinError 10050] A socket operation encountered a dead network",
		10051: "[WinError 10051] A socket operation was attempted to an unreachable network",
		10054: "[WinError 10054] An existing connection was forcibly closed by the remote host",
		10060: "[WinError 10060] A connection attempt failed because the connected party did not properly respond after a period of time, or established connection failed because connected host has failed to respond",
		10061: "[WinError 10061] No connection could be made because the target machine actively refused it",
		10065: "[WinError 10065] A socket operation was attempted to an unreachable host",
		11001: "[WinError 11001] No such host is known",
		11004: "[WinError 11004] The requested name is valid, but no data of the requested type was found",
	} {
		if got := WinErrorString(code); got != want {
			t.Errorf("WinErrorString(%d) = %q, want %q", code, got, want)
		}
		if _, text, _ := strings.Cut(want, "] "); WinErrorText(code) != text {
			t.Errorf("WinErrorText(%d) = %q, want %q", code, WinErrorText(code), text)
		}
	}
}
