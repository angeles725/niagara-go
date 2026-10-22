//go:build !linux

package statcache

import "io/fs"

// Sys is zero off Linux (ok false): stamps there are size, mtime and mode
// (a directory's mtime still changes on every add, remove and rename of a child).
func Sys(fs.FileInfo) (ctime int64, ino, dev uint64, ok bool) { return 0, 0, 0, false }
