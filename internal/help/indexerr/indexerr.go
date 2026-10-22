// Package indexerr holds the error of a required index file that is absent,
// shared by the reader (internal/help) and the builders (internal/helpbuild)
// so both spell one message without either importing the other.
package indexerr

import "fmt"

// Missing is index_cache.MissingIndexError: a required index file is absent,
// so the lookup or the report cannot run (exit 2 with "ERROR: ...").
type Missing struct{ Name, BaseDir string }

func (e *Missing) Error() string {
	return fmt.Sprintf("index indexes/%s is missing under %s; rebuild the indexes "+
		"with: nh rebuild-all", e.Name, e.BaseDir)
}
