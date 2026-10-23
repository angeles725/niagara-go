//go:build !linux

package statcache

import "io/fs"

// Sys is zero off Linux (ok false): a Stamp there is size and mtime, and the
// blockfiles stamps are size, mtime and mode
// (a directory's mtime still changes on every add, remove and rename of a child).
func Sys(fs.FileInfo) (ctime int64, ino, dev uint64, ok bool) { return 0, 0, 0, false }
