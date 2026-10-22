package blocksearch

import (
	"fmt"
	"os"

	"github.com/angeles725/niagara-go/internal/statcache"
)

// statStamp is the fingerprint stamp of a file: size, mtime, ctime and inode
// (findindex.fingerprint; size and mtime where the system does not report the
// other two), "-" when it cannot be stat'ed.
func statStamp(path string) string {
	st, err := os.Stat(path)
	if err != nil {
		return "-"
	}
	if ctime, ino, _, ok := statcache.Sys(st); ok {
		return fmt.Sprintf("%d:%d:%d:%d", st.Size(), st.ModTime().UnixNano(), ctime, ino)
	}
	return fmt.Sprintf("%d:%d", st.Size(), st.ModTime().UnixNano())
}
