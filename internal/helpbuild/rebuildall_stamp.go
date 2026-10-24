package helpbuild

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/angeles725/niagara-go/internal/help/fts"
	"github.com/angeles725/niagara-go/internal/helpbuild/pyfs"
	"github.com/angeles725/niagara-go/internal/pycompat"
	"github.com/angeles725/niagara-go/internal/pyjson"
	"github.com/angeles725/niagara-go/internal/textfold"
)

// The corpus manifest indexes/.manifest.json that rebuild-all stamps and
// nh freshness reads (maintenance._write_manifest, compute_fingerprint):
// {"stamped_utc": isoformat, "sources": {<dir>: {"files", "bytes"} | null},
//  "derived": {<folder>: sha256 hex | null}} as json.dumps(indent=2), written
// atomically. "derived" is Go-only (#199): the content digest of each derived
// folder (fts.DerivedDigest), which freshness compares when a folder was
// touched after the stamp without its content changing.

// sourceDirs are SOURCE_DIRS: the raw corpus folders whose change makes
// the indexes stale, in manifest order.
var sourceDirs = []string{"bajadoc", "source", "guides", "devguide", "docs"}

// sourceExt is SOURCE_EXT: the only extension a folder counts (docs/ holds
// the PDF manuals and the version-diff reports).
var sourceExt = map[string]string{"docs": ".pdf"}

// fingerprintDir is _fingerprint_dir: the file count and total size of the
// tree at path (os.walk: links to folders listed, not entered; a file
// whose size cannot be read, such as a dangling link, not counted); null
// when path is not a folder, or when ext is given and no file has it
// (compared with str.lower()).
func fingerprintDir(path, ext string) pyjson.Value {
	if !pycompat.IsDir(path) {
		return pyjson.Value{}
	}
	var files, total int64
	pycompat.Walk(path, func(dir string, dirs, names []string) []string {
		for _, name := range names {
			if ext != "" && !strings.HasSuffix(textfold.Lower(name), ext) {
				continue
			}
			if st, err := os.Stat(pycompat.Join(dir, name)); err == nil {
				total += st.Size()
				files++
			}
		}
		return dirs
	})
	if ext != "" && files == 0 {
		return pyjson.Value{}
	}
	fp := pyjson.NewObject()
	fp.Set("files", pyjson.IntValue(files))
	fp.Set("bytes", pyjson.IntValue(total))
	return fp
}

// corpusFingerprint is compute_fingerprint: every sourceDirs folder of the
// data root, in order (walked concurrently; the counts do not depend on
// the order).
func corpusFingerprint(root string) pyjson.Value {
	prints := make([]pyjson.Value, len(sourceDirs))
	var wg sync.WaitGroup
	for i, d := range sourceDirs {
		wg.Go(func() { prints[i] = fingerprintDir(pycompat.Join(root, d), sourceExt[d]) })
	}
	wg.Wait()
	out := pyjson.NewObject()
	for i, d := range sourceDirs {
		out.Set(d, prints[i])
	}
	return out
}

// derivedDigests are the content digests of the derived folders of root
// (fts.DerivedNames), null for an absent one, hashed concurrently.
func derivedDigests(root string) pyjson.Value {
	names := fts.DerivedNames(sourceDirs)
	digests := make([]pyjson.Value, len(names))
	var wg sync.WaitGroup
	for i, name := range names {
		wg.Go(func() {
			if d, ok := fts.DerivedDigest(root, name); ok {
				digests[i] = pyjson.String(d)
			}
		})
	}
	wg.Wait()
	out := pyjson.NewObject()
	for i, name := range names {
		out.Set(name, digests[i])
	}
	return out
}

// stampManifest is _write_manifest: the time first, then the fingerprint
// and the derived digests, saved atomically; it returns stamped_utc.
func stampManifest(root string, now time.Time) (string, error) {
	stamped := pyISOFormatUTC(now)
	m := pyjson.NewObject()
	m.Set("stamped_utc", pyjson.String(stamped))
	m.Set("sources", corpusFingerprint(root))
	m.Set("derived", derivedDigests(root))
	if err := pyfs.WriteJSON(pycompat.Join(root, "indexes", ".manifest.json"), m, pyfs.DumpIndent2); err != nil {
		return "", err
	}
	return stamped, nil
}

// pyISOFormatUTC is datetime.now(timezone.utc).isoformat(): microseconds
// (truncated) only when not zero, then "+00:00".
func pyISOFormatUTC(t time.Time) string {
	t = t.UTC()
	s := t.Format("2006-01-02T15:04:05")
	if us := t.Nanosecond() / 1000; us != 0 {
		s += fmt.Sprintf(".%06d", us)
	}
	return s + "+00:00"
}
