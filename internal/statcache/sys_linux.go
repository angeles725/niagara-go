//go:build linux

package statcache

import (
	"io/fs"
	"syscall"
)

// Sys is the ctime (Unix ns), inode and device of fi; ok is false (and the
// values zero) where the system does not report them.
func Sys(fi fs.FileInfo) (ctime int64, ino, dev uint64, ok bool) {
	if sys, isStat := fi.Sys().(*syscall.Stat_t); isStat {
		return int64(sys.Ctim.Sec)*1e9 + int64(sys.Ctim.Nsec), uint64(sys.Ino), uint64(sys.Dev), true // int32 fields on 32-bit
	}
	return 0, 0, 0, false
}
