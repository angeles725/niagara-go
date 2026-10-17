//go:build windows

package licensador

import (
	"crypto/rand"
	"fmt"
	"syscall"
	"unsafe"
)

// stableIdentifiers mirrors _stable_identifiers on Windows: the C:\ volume
// serial (8 upper hex digits, skipped when the call fails), then the node id.
func stableIdentifiers() ([]string, error) {
	var ids []string
	if serial, ok := volumeSerial(`C:\`); ok {
		ids = append(ids, fmt.Sprintf("%08X", serial))
	}
	return append(ids, nodeID()), nil
}

func volumeSerial(root string) (uint32, bool) {
	proc := syscall.NewLazyDLL("kernel32.dll").NewProc("GetVolumeInformationW")
	if proc.Find() != nil {
		return 0, false
	}
	p, err := syscall.UTF16PtrFromString(root)
	if err != nil {
		return 0, false
	}
	var serial uint32
	r, _, _ := proc.Call(uintptr(unsafe.Pointer(p)), 0, 0, uintptr(unsafe.Pointer(&serial)), 0, 0, 0, 0)
	return serial, r != 0
}

func randomNode() string {
	var b [6]byte
	_, _ = rand.Read(b[:])
	b[0] |= 0x01
	return macInt(b[:])
}
