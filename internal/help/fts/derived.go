package fts

// The derived folders of a data root: what rebuild-all writes before it stamps the
// manifest, and which of their files the baseline covers.

import (
	"path/filepath"
	"slices"
	"strings"
)

// RetiredIndexes are the index files the Go port no longer builds nor reads
// (call-index.json, decision I3-5). One left in a data root is harmless but
// misleading: freshness names it so it can be deleted by hand, and never
// deletes it (freshness only reads the data root).
var RetiredIndexes = []string{"call-index.json"}

// DerivedNames are the derived folders: the text sources of the full-text index (Labels)
// that are not raw sources themselves (raw, the SOURCE_DIRS of the caller), and indexes/.
func DerivedNames(raw []string) []string {
	var out []string
	for _, l := range Labels {
		if !slices.Contains(raw, l) {
			out = append(out, l)
		}
	}
	return append(out, "indexes")
}

// WalkDerived calls fn, in WalkFiles order, for every file of the derived folder dir
// (named name) the baseline covers, with its "/" path relative to dir. indexes/ counts
// only its top-level JSON indexes but the manifest and the retired ones: fulltext.sqlite
// has its own fingerprint and is written by the first search after the stamp.
func WalkDerived(dir, name string, fn func(path, rel string)) {
	WalkFiles(dir, func(path string) {
		r, err := filepath.Rel(dir, path)
		if err != nil {
			return
		}
		r = filepath.ToSlash(r)
		if name == "indexes" && (strings.Contains(r, "/") || !strings.HasSuffix(r, ".json") ||
			r == ".manifest.json" || slices.Contains(RetiredIndexes, r)) {
			return
		}
		fn(path, r)
	})
}
