//go:build !linux

package blockfiles

import "os"

// sysStamp is zero off Linux: the stamp is mtime and mode there (a directory's mtime
// still changes on every add, remove and rename of a child).
func sysStamp(os.FileInfo) (ctime int64, ino, dev uint64) { return 0, 0, 0 }
