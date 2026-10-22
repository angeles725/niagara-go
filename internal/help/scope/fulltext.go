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
// then use the index the first one wrote. A lock left by a process that died
// (Ctrl-C) is taken over at once (lock_unix.go / lock_windows.go check its
// pid), one older than lockStale too. A search waits lockWait for a live
// holder, longer while the holder keeps touching the lock: a corpus rebuild
// (nh rebuild-all) holds the same lock with a heartbeat for its whole run.

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/angeles725/niagara-go/internal/help/fts"
	ftbuild "github.com/angeles725/niagara-go/internal/helpbuild/fulltext"
	"github.com/angeles725/niagara-go/internal/pycompat"
)

// Lock timing; tests shorten them.
var (
	lockPoll  = 100 * time.Millisecond // how often a waiting process retries the lock
	lockWait  = 2 * time.Minute        // how long it waits before building anyway
	lockStale = 10 * time.Minute       // a lock older than this is taken over, live pid or not
	// lockFollowMax caps how long a search follows a heartbeating rebuild
	// (a holder whose heartbeat outlives its hung work): then it builds anyway.
	lockFollowMax = 30 * time.Minute
	// guardStale is the age after which a takeover guard is abandoned.
	guardStale = time.Minute
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

// OpenFulltext is the fts.Opener of nh's searches: ensure, then the
// up-to-date database, opened at the point fulltext.search opens it. After
// any rebuild notice it notes on stderr the query words searched as their
// English glossary terms (fts.Rewrites), so a Spanish query is never
// rewritten silently; nh and the in-process searchdoor share this line.
func OpenFulltext(stderr io.Writer) fts.Opener {
	return func(baseDir, _, query string) (*fts.Index, error) {
		path, _, err := ensure(baseDir, stderr)
		if err != nil {
			return nil, err
		}
		if rw := fts.Rewrites(query); len(rw) > 0 {
			fmt.Fprintf(stderr, "note: searched as English (glossary): %s\n", strings.Join(rw, "; "))
		}
		return fts.OpenPath(path)
	}
}

// ensure is open_index up to the open: the path of an up-to-date index,
// built here when needed (rebuilt).
func ensure(baseDir string, stderr io.Writer) (path string, rebuilt bool, err error) {
	current := fts.CachedFingerprint(baseDir, fts.SourceDirs(baseDir))
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
		p, built, err := buildAt(baseDir, out, current, announce, stderr)
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
// returns that index's path and no log lines. Waiting on a corpus rebuild is
// noted once on stderr.
func buildAt(baseDir, out, current string, announce func(), stderr io.Writer) (path string, log []string, err error) {
	if err := os.MkdirAll(filepath.Dir(out), 0o777); err != nil {
		return "", nil, err
	}
	lock := out + ".lock"
	mine, err := acquireFollowing(lock, func(holder []byte) { fmt.Fprint(stderr, waitingNote(lock, holder)) })
	if err != nil {
		return "", nil, err
	}
	if mine != nil {
		defer release(lock, mine)
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

// acquire creates the lock file (O_EXCL) and returns what it wrote in it:
// "<pid> <GOOS> <host> <time>". A lock whose holder is dead (stale) is taken
// over at once; after waiting lockWait for a live one it returns nil, and
// the caller builds anyway (a build writes a temp file and renames it, so
// concurrent builds stay safe). An error means the lock cannot be created:
// the location is not writable.
func acquire(lock string) (mine []byte, err error) { return acquireLock(lock, nil) }

// acquireFollowing is acquire for a search about to build the index: past
// lockWait it keeps waiting while the same lock's mtime moved within the
// last lockWait, the heartbeat of a corpus rebuild (WithBuildLock), so the
// search never indexes a half-rewritten corpus; onWait runs once, when
// that extra wait starts. A lock that stops moving (a plain index build, a
// hung or remote holder) gets acquire's build-anyway lockWait after its last
// move, a new holder gets a fresh lockWait, a dead holder's lock is still
// taken over (stale), and after lockFollowMax the search builds anyway.
func acquireFollowing(lock string, onWait func(holder []byte)) ([]byte, error) {
	return acquireLock(lock, onWait)
}

func acquireLock(lock string, onWait func(holder []byte)) (mine []byte, err error) {
	start := time.Now()
	deadline := start.Add(lockWait)
	var lastSeen []byte
	var lastMtime, lastMove time.Time
	waiting := false
	for {
		content := lockContent()
		f, err := os.OpenFile(lock, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o666)
		if err == nil {
			_, werr := f.Write(content)
			if cerr := f.Close(); werr == nil {
				werr = cerr
			}
			if werr != nil {
				os.Remove(lock) //nolint:errcheck // best effort: the write error is reported; a lock left behind goes stale
				return nil, werr
			}
			return content, nil
		}
		if !errors.Is(err, fs.ErrExist) {
			return nil, err
		}
		seen, rerr := os.ReadFile(lock)
		st, serr := os.Stat(lock)
		if errors.Is(rerr, fs.ErrNotExist) || errors.Is(serr, fs.ErrNotExist) {
			continue // released meanwhile
		}
		if rerr == nil && serr == nil && stale(seen, st.ModTime()) && takeOver(lock, seen) {
			continue
		}
		now := time.Now()
		if onWait != nil && now.Sub(start) > lockFollowMax {
			return nil, nil // checked first: changing holders reset the deadline below
		}
		if rerr == nil && serr == nil && len(seen) > 0 { // empty: a lock being written
			if !bytes.Equal(seen, lastSeen) { // another holder: its moves count from now
				if lastSeen != nil && onWait != nil {
					deadline = now.Add(lockWait) // and it gets a full wait of its own
				}
				lastSeen, lastMtime, lastMove = seen, st.ModTime(), time.Time{}
			} else if st.ModTime().After(lastMtime) {
				lastMtime, lastMove = st.ModTime(), now
			}
		}
		if now.After(deadline) {
			if onWait == nil || lastMove.IsZero() || now.Sub(lastMove) > lockWait {
				return nil, nil
			}
			if !waiting {
				waiting = true
				onWait(lastSeen)
			}
		}
		time.Sleep(lockPoll)
	}
}

// lockContent identifies this process's lock: pid, OS and host (a pid only
// means something on the host that wrote it) and the time to the nanosecond,
// so no two locks have the same content.
func lockContent() []byte {
	host, _ := os.Hostname()
	if host == "" || strings.ContainsAny(host, " \n") {
		host = "-"
	}
	return fmt.Appendf(nil, "%d %s %s %s\n", os.Getpid(), runtime.GOOS, host,
		time.Now().UTC().Format(time.RFC3339Nano))
}

// stale reports whether a lock with this content and modification time was
// left by a process that is gone: it is older than lockStale, or it names a
// pid of this OS and host that is not running (Ctrl-C ends nh without
// running its deferred release). Anything unclear counts as live.
func stale(content []byte, mtime time.Time) bool {
	if time.Since(mtime) > lockStale {
		return true
	}
	f := strings.Fields(string(content))
	host, _ := os.Hostname()
	if len(f) < 4 || f[1] != runtime.GOOS || host == "" || f[2] != host {
		return false
	}
	pid, err := strconv.Atoi(f[0])
	if err != nil || pid == os.Getpid() {
		return false
	}
	alive, known := pidAlive(pid)
	return known && !alive
}

// takeOver removes the stale lock whose content was seen, so that of several
// waiters only one does: the one that creates the guard file <lock>.takeover
// (O_EXCL) removes the lock if it still has the content seen; the others
// report false and retry. (Renaming the lock away is not exclusive on
// Windows: two concurrent renames of one file can both succeed.) A guard
// older than guardStale, left by a process that died in these few
// statements, is removed.
func takeOver(lock string, seen []byte) bool {
	guard := lock + ".takeover"
	f, err := os.OpenFile(guard, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o666)
	if err != nil {
		if st, serr := os.Stat(guard); serr == nil && time.Since(st.ModTime()) > guardStale {
			os.Remove(guard) //nolint:errcheck // best effort: another waiter may have removed the stale guard first
		}
		return false
	}
	f.Close()              //nolint:errcheck // best effort: the guard is used by its existence, never written
	defer os.Remove(guard) //nolint:errcheck // best effort: a guard left behind ages past guardStale and is removed
	if got, err := os.ReadFile(lock); err != nil || !bytes.Equal(got, seen) {
		return false
	}
	return os.Remove(lock) == nil
}

// release removes the lock if it still holds this process's content (a lock
// taken over after lockStale belongs to another process).
func release(lock string, mine []byte) {
	if got, err := os.ReadFile(lock); err == nil && bytes.Equal(got, mine) {
		os.Remove(lock) //nolint:errcheck // best effort: a lock left behind is taken over after lockStale
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
			return fmt.Sprintf("[Errno %d] %s: %s", int(errno), msg, pycompat.FSRepr(pe.Path))
		case pe.Op == "open":
			return "unable to open database file"
		}
	}
	return err.Error()
}

// absPath is os.path.abspath, the data root's identity in the failure memo.
func absPath(p string) string {
	if a, err := filepath.Abs(p); err == nil {
		return a
	}
	return p
}

// waitingNote is the stderr line of a search that waits past lockWait for a
// corpus rebuild still touching its lock.
func waitingNote(lock string, holder []byte) string {
	return fmt.Sprintf("full-text: waiting for a corpus rebuild to finish (%s held by %s)\n",
		lock, strings.TrimSpace(string(holder)))
}
