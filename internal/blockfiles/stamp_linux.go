//go:build linux

package blockfiles

import (
	"os"
	"syscall"
)

// sysStamp is the ctime, inode and device of st.
func sysStamp(st os.FileInfo) (ctime int64, ino, dev uint64) {
	if sys, ok := st.Sys().(*syscall.Stat_t); ok {
		return int64(sys.Ctim.Sec)*1e9 + int64(sys.Ctim.Nsec), uint64(sys.Ino), uint64(sys.Dev) // int32 fields on 32-bit
	}
	return 0, 0, 0
}
