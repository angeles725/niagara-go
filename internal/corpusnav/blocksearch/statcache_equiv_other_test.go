//go:build !linux

package blocksearch

import (
	"fmt"
	"os"
)

// legacyStatStamp is stamp_other.go before internal/statcache (PR #179), verbatim.
func legacyStatStamp(path string) string {
	st, err := os.Stat(path)
	if err != nil {
		return "-"
	}
	return fmt.Sprintf("%d:%d", st.Size(), st.ModTime().UnixNano())
}
