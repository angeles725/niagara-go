//go:build !linux

package stattest

import "io/fs"

// ctimeOf reports no ctime off Linux, as statcache.Sys.
func ctimeOf(fs.FileInfo) (int64, bool) { return 0, false }
