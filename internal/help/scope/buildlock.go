package scope

// WithBuildLock runs a corpus rebuild (nh rebuild-all) under the lock file
// the full-text build takes (indexes/fulltext.sqlite.lock, see fulltext.go),
// so a search that needs to build the full-text index waits for the
// rebuild instead of indexing a half-rewritten corpus, and two rebuilds of
// one data root do not run at once.
//
// A rebuild can run longer than lockStale (about ten minutes for a large
// version), so while fn runs a heartbeat touches the lock's modification
// time every heartbeatEvery (lockWait/4, 30 s): waiters never see it as
// stale, and a waiting search sees it move well before its lockWait ends. A lock whose holder
// died stops being touched and is still taken over after lockStale. The
// heartbeat only changes the mtime (os.Chtimes, never a new file), and is
// stopped and joined before the lock is released, so it never touches the
// next holder's lock after a normal release. Before each touch it checks
// that the lock still holds this process's content; the check and the
// touch are two steps, so one race remains: when this process stalled for
// longer than lockStale (no heartbeat ran), a waiter may take the lock over
// between the check and the touch, and the touch then refreshes the new
// holder's lock once. That holder is alive, so the only effect is that its
// lock looks fresh a little longer; the next check sees the other content
// and the heartbeat stops.
//
// The build cannot tell a waiter to back off, so when fn returns the lock
// is read once more: content that is no longer this process's (taken over,
// or removed) is reported as an error, since another build may have run
// alongside this one; release then leaves the other holder's lock alone.
//
// A touch that fails (os.Chtimes refused) is retried on the next tick; when
// heartbeatMaxFailures touches fail in a row the lock has gone unrefreshed
// for most of lockStale, so a waiter may have taken it over mid-build.
// fn cannot be interrupted, so the build runs to its end, and then a
// successful build is reported as an error: its output is not trusted.

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	ftbuild "github.com/angeles725/niagara-go/internal/helpbuild/fulltext"
)

// LockBusyError is a build lock still held by a live process after lockWait.
type LockBusyError struct {
	Lock   string // the lock file
	Holder string // its content: "<pid> <GOOS> <host> <time>"
}

func (e *LockBusyError) Error() string {
	return fmt.Sprintf("the build lock %s is held by another process (%s); run again when it has finished",
		e.Lock, e.Holder)
}

// BuildLockPath is the lock file of baseDir's corpus builds.
func BuildLockPath(baseDir string) string { return ftbuild.DefaultOutput(baseDir) + ".lock" }

// WithBuildLock runs fn holding the build lock of baseDir and returns fn's
// error unchanged; when fn succeeded but the lock was taken over, or its
// heartbeat failed (see the header comment), the error names the lock. The lock is released when fn returns or panics. When a
// live process still holds the lock after lockWait, fn does not run and
// the error is *LockBusyError; an error creating the lock (indexes/ not
// writable) is returned as is.
func WithBuildLock(baseDir string, fn func() error) error {
	lock := BuildLockPath(baseDir)
	if err := os.MkdirAll(filepath.Dir(lock), 0o777); err != nil {
		return err
	}
	mine, err := acquire(lock)
	if err != nil {
		return err
	}
	if mine == nil {
		holder, _ := os.ReadFile(lock)
		return &LockBusyError{Lock: lock, Holder: string(bytes.TrimSpace(holder))}
	}
	ticks, stopTicks := heartbeatTicks(heartbeatEvery())
	defer stopTicks()
	stop, _ := heartbeat(lock, mine, ticks)
	defer func() {
		stop() // joined before the release, also on a panic; a second stop is a no-op
		release(lock, mine)
	}()
	err = fn()
	touchErr := stop()
	if err != nil {
		return err
	}
	if lost := takenOver(lock, mine); lost != nil {
		return lost
	}
	if touchErr != nil {
		err = fmt.Errorf("the build lock %s could not be refreshed during the build (%w); "+
			"another process may have taken it over, so run again", lock, touchErr)
	}
	return err
}

// takenOver is the error of a lock that no longer holds mine when the build ends: another
// process took it over (it saw the lock as stale) or removed it, so the build may have run
// alongside another one. nil while the lock is still this process's.
func takenOver(lock string, mine []byte) error {
	got, err := os.ReadFile(lock)
	switch {
	case err != nil:
		return fmt.Errorf("the build lock %s was removed by another process during the build (%w); run again",
			lock, err)
	case !bytes.Equal(got, mine):
		return fmt.Errorf("the build lock %s was taken over during the build by another process (%s); run again",
			lock, bytes.TrimSpace(got))
	}
	return nil
}

// heartbeatEvery is the heartbeat interval: a quarter of lockWait, or of
// lockStale when that is shorter (only in tests), so the lock never ages
// past lockStale while it is held.
func heartbeatEvery() time.Duration { return min(lockWait, lockStale) / 4 }

// heartbeatTicks makes the heartbeat's ticks, one per interval, and the
// function that stops them; tests replace it to drive the ticks by hand.
var heartbeatTicks = func(interval time.Duration) (<-chan time.Time, func()) {
	t := time.NewTicker(interval)
	return t.C, t.Stop
}

// chtimes is the heartbeat's touch; tests replace it to make touches fail.
var chtimes = os.Chtimes

// heartbeatMaxFailures is how many touches in a row may fail before the
// heartbeat reports it: three ticks (3/4 of min(lockWait, lockStale))
// without a refresh, before a waiter can see the lock as stale.
const heartbeatMaxFailures = 3

// heartbeat touches lock on each tick while it holds mine (see the race in
// the header comment). stop ends the goroutine, waits for it and returns
// the touch error once heartbeatMaxFailures touches have failed in a row
// (nil otherwise); ended is closed when the goroutine has returned
// (stopped, or the lock no longer holds mine).
func heartbeat(lock string, mine []byte, ticks <-chan time.Time) (stop func() error, ended <-chan struct{}) {
	done, exited := make(chan struct{}), make(chan struct{})
	var failed error // written by the goroutine, read after exited
	go func() {
		defer close(exited)
		consecutive := 0
		for {
			select {
			case <-done:
				return
			case <-ticks:
				if got, err := os.ReadFile(lock); err != nil || !bytes.Equal(got, mine) {
					return // released or taken over
				}
				now := time.Now()
				if err := chtimes(lock, now, now); err != nil {
					if consecutive++; consecutive >= heartbeatMaxFailures && failed == nil {
						failed = err
					}
					continue
				}
				consecutive = 0
			}
		}
	}()
	var once sync.Once
	return func() error {
		once.Do(func() { close(done) })
		<-exited
		return failed
	}, exited
}
