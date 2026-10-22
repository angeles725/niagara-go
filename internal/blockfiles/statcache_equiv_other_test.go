//go:build !linux

package blockfiles

import "os"

// legacySysStamp is stamp_other.go before internal/statcache (PR #179), verbatim.
func legacySysStamp(os.FileInfo) (ctime int64, ino, dev uint64) { return 0, 0, 0 }
