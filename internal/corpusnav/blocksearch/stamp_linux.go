//go:build linux

package blocksearch

import (
	"fmt"
	"os"
	"syscall"
)

// statStamp is the fingerprint stamp of a file: size, mtime, ctime and inode
// (findindex.fingerprint), "-" when it cannot be stat'ed.
func statStamp(path string) string {
	st, err := os.Stat(path)
	if err != nil {
		return "-"
	}
	if sys, ok := st.Sys().(*syscall.Stat_t); ok {
		return fmt.Sprintf("%d:%d:%d:%d", st.Size(), st.ModTime().UnixNano(),
			int64(sys.Ctim.Sec)*1e9+int64(sys.Ctim.Nsec), sys.Ino)
	}
	return fmt.Sprintf("%d:%d", st.Size(), st.ModTime().UnixNano())
}
