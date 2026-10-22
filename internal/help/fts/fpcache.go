package fts

// The fingerprint memo: Fingerprint walks every text source (about 2,900
// folders and 15,600 files on 4.15) before each search. CachedFingerprint
// gives the same value from fingerprint.gob, kept per data root beside the
// cache copy of the index (CachePath's folder): per source the folders it
// walked with their mtimes, its files in IterFiles order and their stats.
//
// A source is walked again only when one of its folders changed: adding,
// removing or renaming an entry changes the folder's mtime, and a folder
// that could not be read, a source reached through another link target, or
// a folder modified less than fpRecent before the walk (it may change again
// within the same timestamp tick) is never trusted. A file changed in place
// (the extract of versions add rewrites files with O_TRUNC, an editor may
// too) leaves its folder alone, so every file is still stat'ed - in parallel,
// without listing a folder - and the part of a source is hashed again
// whenever a stat differs. The value is therefore Fingerprint's exactly; the
// memo only saves the walk.
//
// The file follows the gob caches of module-nav (internal/statcache): gob, written atomically with mode 0600 after a change,
// ignored when malformed or of another format, never an error.

import (
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	"github.com/angeles725/niagara-go/internal/statcache"
)

// fpFormat is the memo's format; a memo of another format is ignored.
const fpFormat = 1

// fpRecent is how recent a folder mtime is not trusted to stay unchanged.
const fpRecent = 2 * time.Second

// fpStatWorkers is how many goroutines stat the files of a source.
const fpStatWorkers = 4

// fpWalks counts the sources CachedFingerprint walked (for the tests).
var fpWalks atomic.Int64

type fpMemo struct {
	Format  int
	Engine  string
	Sources []fpSource
}

// fpSource is one source of the memo. Trusted is false when its folders
// must be listed again next time.
type fpSource struct {
	Label, Dir, Real string
	Trusted          bool
	Dirs             []string
	DirMtimes        []int64
	Files            []string // relative to Dir
	Stats            []fileStat
	Part             string
}

// fpMemoPath is the memo of baseDir: fingerprint.gob in CachePath's folder.
func fpMemoPath(baseDir string) string {
	return filepath.Join(filepath.Dir(CachePath(baseDir)), "fingerprint.gob")
}

// CachedFingerprint is Fingerprint(dirs) for the sources of baseDir, read
// through the memo (see above); a memo that cannot be written only costs
// the next call its walk.
func CachedFingerprint(baseDir string, dirs []SourceDir) string {
	path := fpMemoPath(baseDir)
	old := map[[2]string]*fpSource{}
	if m := loadFpMemo(path); m != nil {
		for i := range m.Sources {
			s := &m.Sources[i]
			old[[2]string{s.Label, s.Dir}] = s
		}
	}
	memo := &fpMemo{Format: fpFormat, Engine: EngineVersion}
	parts := []string{"engine=" + EngineVersion}
	changed := len(old) != len(dirs)
	for _, d := range dirs {
		s, ok := old[[2]string{d.Label, d.Dir}]
		if !ok || !s.unchanged() {
			s, changed = walkSource(d), true
		}
		abs := make([]string, len(s.Files))
		for i, f := range s.Files {
			abs[i] = joinUnder(d.Dir, f)
		}
		stats := statFiles(abs, fpStatWorkers)
		if !equalStats(stats, s.Stats) || s.Part == "" {
			s.Stats, s.Part, changed = stats, sourcePart(d, abs, stats), true
		}
		parts = append(parts, s.Part)
		memo.Sources = append(memo.Sources, *s)
	}
	if changed {
		writeFpMemo(path, memo)
	}
	return strings.Join(parts, "|")
}

// unchanged reports whether the folders of s can be trusted to list what
// they listed when s was walked.
func (s *fpSource) unchanged() bool {
	if !s.Trusted || realPath(s.Dir) != s.Real {
		return false
	}
	for i, d := range s.Dirs {
		st, err := os.Stat(d)
		if err != nil || st.ModTime().UnixNano() != s.DirMtimes[i] {
			return false
		}
	}
	return true
}

// walkSource lists the files of d as IterFiles does and records its folders.
func walkSource(d SourceDir) *fpSource {
	fpWalks.Add(1)
	start := time.Now()
	s := &fpSource{Label: d.Label, Dir: d.Dir, Real: realPath(d.Dir), Trusted: true}
	files := iterFiles(d.Label, d.Dir, func(dir string, listed bool) {
		st, err := os.Stat(dir)
		if !listed || err != nil || st.ModTime().After(start.Add(-fpRecent)) {
			s.Trusted = false
			return
		}
		s.Dirs = append(s.Dirs, dir)
		s.DirMtimes = append(s.DirMtimes, st.ModTime().UnixNano())
	})
	s.Files = make([]string, len(files))
	for i, f := range files {
		if r, err := filepath.Rel(d.Dir, f); err == nil {
			s.Files[i] = r
		} else {
			s.Files[i], s.Trusted = f, false
		}
	}
	return s
}

// realPath is dir with every link resolved ("" when it cannot be).
func realPath(dir string) string {
	r, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return ""
	}
	return r
}

// joinUnder is the path of a memo file name: relative to dir, or absolute
// when it could not be made relative (walkSource then distrusts the source).
func joinUnder(dir, name string) string {
	if filepath.IsAbs(name) {
		return name
	}
	return filepath.Join(dir, name)
}

// statFiles stats the files with that many goroutines (statcache.Files); a
// file that cannot be stat'ed is the zero fileStat.
func statFiles(files []string, workers int) []fileStat {
	out := make([]fileStat, len(files))
	for i, s := range statcache.Files(files, workers) {
		if s.Exists() {
			out[i] = fileStat{s.Size, s.MtimeNs, true}
		}
	}
	return out
}

func equalStats(a, b []fileStat) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// loadFpMemo reads the memo; nil when it is missing, malformed or of
// another format or engine.
func loadFpMemo(path string) *fpMemo {
	var m fpMemo
	if !statcache.ReadGob(path, &m) || m.Format != fpFormat || m.Engine != EngineVersion {
		return nil
	}
	for _, s := range m.Sources {
		if len(s.Dirs) != len(s.DirMtimes) || len(s.Files) != len(s.Stats) {
			return nil
		}
	}
	return &m
}

// writeFpMemo writes the memo atomically (mode 0600); a failure is ignored.
func writeFpMemo(path string, m *fpMemo) { statcache.WriteGob(path, m) }
