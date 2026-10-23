//go:build linux

package stattest

import (
	"io/fs"
	"syscall"
)

// ctimeOf is the ctime (Unix ns) of fi, as statcache.Sys reads it (stattest
// cannot import statcache, whose own tests use it).
func ctimeOf(fi fs.FileInfo) (int64, bool) {
	if sys, ok := fi.Sys().(*syscall.Stat_t); ok {
		return int64(sys.Ctim.Sec)*1e9 + int64(sys.Ctim.Nsec), true // int32 fields on 32-bit
	}
	return 0, false
}
