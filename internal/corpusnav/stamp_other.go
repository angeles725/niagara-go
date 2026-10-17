//go:build !linux

package corpusnav

import (
	"fmt"
	"os"
)

// statStamp is the fingerprint stamp of a file: size and mtime (ctime and inode are
// read on Linux only), "-" when it cannot be stat'ed.
func statStamp(path string) string {
	st, err := os.Stat(path)
	if err != nil {
		return "-"
	}
	return fmt.Sprintf("%d:%d", st.Size(), st.ModTime().UnixNano())
}
