package scope

// Rebuilding a missing or stale full-text index before a search, as
// fulltext_index.open_index does (decision D38, superseding D21's exit 2):
// the building notice, then the index is built at indexes/fulltext.sqlite or,
// when indexes/ is not writable, in the user cache directory, each failed
// location noted on stderr. Python's last resort, an in-memory index for the
// process, has no counterpart: with no writable location the search fails
// with *UnwritableIndexError (exit 2), an intended difference.
//
// Unlike Python, a build holds a lock file next to the database, so
// concurrent nh / nsearch processes build an index once: the others wait,
// then use the index the first one wrote.

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/angeles725/niagara-go/internal/help/fts"
	ftbuild "github.com/angeles725/niagara-go/internal/helpbuild/fulltext"
)

// Lock timing; tests shorten them.
var (
	lockPoll  = 100 * time.Millisecond // how often a waiting process retries the lock
	lockWait  = 2 * time.Minute        // how long it waits before building anyway
	lockStale = 10 * time.Minute       // a lock older than this is taken over
)

// UnwritableIndexError is a missing or stale full-text index that cannot be
// built: neither indexes/ nor the user cache directory is writable (Python
// would keep the index in memory for the process).
type UnwritableIndexError struct {
	BaseDir string
	Missing bool // no index file exists (else they are out of date)
}

func (e *UnwritableIndexError) Error() string {
	state := "is out of date"
	if e.Missing {
		state = "is missing"
	}
	return fmt.Sprintf("full-text index indexes/%s under %s %s and cannot be built: neither "+
		"indexes/ nor the user cache directory is writable", fts.DBName, e.BaseDir, state)
}

// failed remembers the data roots (and fingerprints) no location could hold
// an index for, so a later search of the same process fails at once, without
// repeating the notes (Python's note() prints each one once per process).
var failed sync.Map

// EnsureFulltext makes sure baseDir has an up-to-date full-text index before
// a search: when indexes/fulltext.sqlite and the cache copy are both missing
// or stale it builds one, printing Python's lines on stderr, and reports
// rebuilt. An error means no index could be built (*UnwritableIndexError).
func EnsureFulltext(baseDir string, stderr io.Writer) (rebuilt bool, err error) {
	_, rebuilt, err = ensure(baseDir, stderr)
	return rebuilt, err
}

// OpenFulltext is the fts.Opener of nh's searches: EnsureFulltext, then the
// up-to-date database, opened at the point fulltext.search opens it.
func OpenFulltext(stderr io.Writer) fts.Opener {
	return func(baseDir, _ string) (*fts.Index, error) {
		path, _, err := ensure(baseDir, stderr)
		if err != nil {
			return nil, err
		}
		return fts.OpenPath(path)
	}
}

// ensure is open_index up to the open: the path of an up-to-date index,
// built here when needed (rebuilt).
func ensure(baseDir string, stderr io.Writer) (path string, rebuilt bool, err error) {
	current := fts.Fingerprint(fts.SourceDirs(baseDir))
	path, missing := fts.FreshPath(baseDir, current)
	if path != "" {
		return path, false, nil
	}
	key := absPath(baseDir) + "\x00" + current
	if e, ok := failed.Load(key); ok {
		return "", false, e.(error)
	}
	announced := false
	announce := func() {
		if !announced {
			announced = true
			fmt.Fprintf(stderr, "building the full-text index for %s (one-time, after any corpus change)...\n", baseDir)
		}
	}
	// open_index's places, spelled as Python names them in its notes.
	for _, out := range []string{ftbuild.DefaultOutput(baseDir), fts.CachePath(baseDir)} {
		dir := strings.TrimSuffix(out, string(filepath.Separator)+fts.DBName)
		p, built, err := buildAt(baseDir, out, current, announce)
		if err == nil {
			for _, line := range built {
				fmt.Fprintln(stderr, line)
			}
			return p, len(built) > 0, nil
		}
		announce()
		fmt.Fprintf(stderr, "full-text: %s is not writable (%s); trying the next location\n", dir, pyReason(err))
	}
	err = &UnwritableIndexError{BaseDir: baseDir, Missing: missing}
	failed.Store(key, err)
	return "", false, err
}

// buildAt builds the index of baseDir at out under its lock. When another
// process wrote an up-to-date index while this one waited for the lock, it
// returns that index's path and no log lines.
func buildAt(baseDir, out, current string, announce func()) (path string, log []string, err error) {
	if err := os.MkdirAll(filepath.Dir(out), 0o777); err != nil {
		return "", nil, err
	}
	lock := out + ".lock"
	held, err := acquire(lock)
	if err != nil {
		return "", nil, err
	}
	if held {
		defer os.Remove(lock)
	}
	if p, _ := fts.FreshPath(baseDir, current); p != "" {
		return p, nil, nil
	}
	announce()
	opts := ftbuild.Options{}
	if out != ftbuild.DefaultOutput(baseDir) {
		opts.Output = out
	}
	res, err := ftbuild.Build(baseDir, opts)
	if err != nil {
		return "", nil, err
	}
	return res.Path, res.Log, nil
}

// acquire creates the lock file (O_EXCL; it holds the pid and the time). A
// lock older than lockStale is taken over; after waiting lockWait for a live
// one it returns held false, and the caller builds anyway (a build writes a
// temp file and renames it, so concurrent builds stay safe). An error means
// the lock cannot be created: the location is not writable.
func acquire(lock string) (held bool, err error) {
	deadline := time.Now().Add(lockWait)
	for {
		f, err := os.OpenFile(lock, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o666)
		if err == nil {
			_, werr := fmt.Fprintf(f, "%d %s\n", os.Getpid(), time.Now().UTC().Format(time.RFC3339))
			if cerr := f.Close(); werr == nil {
				werr = cerr
			}
			if werr != nil {
				os.Remove(lock)
				return false, werr
			}
			return true, nil
		}
		if !errors.Is(err, fs.ErrExist) {
			return false, err
		}
		if st, serr := os.Stat(lock); serr == nil && time.Since(st.ModTime()) > lockStale {
			os.Remove(lock)
			continue
		}
		if time.Now().After(deadline) {
			return false, nil
		}
		time.Sleep(lockPoll)
	}
}

// pyReason is the exception text Python's note shows for a location it
// cannot write: os.makedirs's "[Errno N] Message: 'dir'", or SQLite's
// "unable to open database file" for a database it cannot create.
func pyReason(err error) string {
	var pe *fs.PathError
	if errors.As(err, &pe) {
		var errno syscall.Errno
		switch {
		case pe.Op == "mkdir" && errors.As(pe.Err, &errno):
			msg := errno.Error()
			if msg != "" {
				msg = strings.ToUpper(msg[:1]) + msg[1:]
			}
			return fmt.Sprintf("[Errno %d] %s: %s", int(errno), msg, pyRepr(pe.Path))
		case pe.Op == "open":
			return "unable to open database file"
		}
	}
	return err.Error()
}

// pyRepr is Python's repr of a str for the printable paths it shows:
// single quotes, or double quotes when the text has a single quote only.
func pyRepr(s string) string {
	if strings.Contains(s, "'") && !strings.Contains(s, `"`) {
		return `"` + strings.ReplaceAll(s, `\`, `\\`) + `"`
	}
	return "'" + strings.NewReplacer(`\`, `\\`, "'", `\'`).Replace(s) + "'"
}

// absPath is os.path.abspath, the data root's identity in the failure memo.
func absPath(p string) string {
	if a, err := filepath.Abs(p); err == nil {
		return a
	}
	return p
}
