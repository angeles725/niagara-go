package fts

// The derived folders of a data root: what rebuild-all writes before it stamps the
// manifest, read by both sides of the stamp. helpbuild records a content digest of each
// at stamp time; nh freshness compares it when a folder was touched after the stamp
// (#199: a git checkout rewrites unchanged files), so both sides select the same files.

import (
	"crypto/sha256"
	"encoding/hex"
	"path/filepath"
	"slices"
	"strings"

	"github.com/angeles725/niagara-go/internal/pycompat"
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

// DerivedDigest is the content digest of the derived folder name of root: the hex sha256
// of one "<rel>\x00<sha256 of the file>\n" line per WalkDerived file, in walk order. A
// file that cannot be read (a dangling link) is left out, as the fingerprints leave it.
// ok is false when the folder is absent.
func DerivedDigest(root, name string) (digest string, ok bool) {
	dir := filepath.Join(root, name)
	if !pycompat.IsDir(dir) {
		return "", false
	}
	h := sha256.New()
	WalkDerived(dir, name, func(path, rel string) {
		if sum, err := pycompat.FileSHA256(path); err == nil {
			h.Write([]byte(rel + "\x00" + sum + "\n"))
		}
	})
	return hex.EncodeToString(h.Sum(nil)), true
}
