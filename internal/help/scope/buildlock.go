package scope

// WithBuildLock runs a corpus rebuild (nh rebuild-all) under the lock file
// the full-text build takes (indexes/fulltext.sqlite.lock, see fulltext.go),
// so a search that needs to build the full-text index waits for the
// rebuild instead of indexing a half-rewritten corpus, and two rebuilds of
// one data root do not run at once.
//
// A rebuild can run longer than lockStale (about ten minutes for a large
// version), so while fn runs a heartbeat touches the lock's modification
// time every lockStale/4: waiters never see it as stale. A lock whose holder
// died stops being touched and is still taken over after lockStale. The
// heartbeat only changes the mtime (os.Chtimes, never a new file), touches
// the lock only while it still holds this process's content, and is
// stopped and joined before the lock is released, so it can never refresh
// the lock of the next holder.

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
// error unchanged. The lock is released when fn returns or panics. When a
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
	stop := heartbeat(lock, mine, lockStale/4)
	defer func() {
		stop() // joined before the release
		release(lock, mine)
	}()
	return fn()
}

// heartbeat touches lock every interval while it holds mine; the returned
// function stops the goroutine and waits for it to end.
func heartbeat(lock string, mine []byte, interval time.Duration) (stop func()) {
	done := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		tick := time.NewTicker(interval)
		defer tick.Stop()
		for {
			select {
			case <-done:
				return
			case <-tick.C:
				if got, err := os.ReadFile(lock); err != nil || !bytes.Equal(got, mine) {
					return // released or taken over: never touch another holder's lock
				}
				now := time.Now()
				os.Chtimes(lock, now, now)
			}
		}
	}()
	var once sync.Once
	return func() {
		once.Do(func() { close(done) })
		wg.Wait()
	}
}
