package scope

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

// lockRoot is a data root with an indexes/ folder and its build lock path.
func lockRoot(t *testing.T) (root, lock string) {
	t.Helper()
	root = t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "indexes"), 0o755); err != nil {
		t.Fatal(err)
	}
	return root, BuildLockPath(root)
}

func gone(t *testing.T, lock string) {
	t.Helper()
	if _, err := os.Stat(lock); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("lock %s left behind: %v", lock, err)
	}
}

func TestWithBuildLockReturnsFnErrorAndReleases(t *testing.T) {
	root, lock := lockRoot(t)
	boom := errors.New("builder failed")
	held := false
	err := WithBuildLock(root, func() error {
		_, serr := os.Stat(lock)
		held = serr == nil
		return boom
	})
	if err != boom || !held {
		t.Errorf("err %v (want the fn's error unchanged), lock held during fn: %v", err, held)
	}
	gone(t, lock)
	if err := WithBuildLock(root, func() error { return nil }); err != nil {
		t.Errorf("second run: %v", err)
	}
}

func TestWithBuildLockReleasesOnPanic(t *testing.T) {
	root, lock := lockRoot(t)
	func() {
		defer func() {
			if r := recover(); r != "boom" {
				t.Errorf("recovered %v", r)
			}
		}()
		WithBuildLock(root, func() error { panic("boom") })
	}()
	gone(t, lock)
}

// TestWithBuildLockHeartbeatKeepsALongHolder: a holder that runs several
// times lockStale keeps its lock fresh; a waiter gets the lock only after
// fn has returned.
func TestWithBuildLockHeartbeatKeepsALongHolder(t *testing.T) {
	lockTiming(t, 5*time.Millisecond, 30*time.Second, 120*time.Millisecond)
	root, lock := lockRoot(t)
	started := make(chan struct{})
	var holderDone atomic.Bool
	var freshest time.Duration
	errc := make(chan error, 1)
	go func() {
		errc <- WithBuildLock(root, func() error {
			close(started)
			for end := time.Now().Add(600 * time.Millisecond); time.Now().Before(end); {
				time.Sleep(20 * time.Millisecond)
				if st, err := os.Stat(lock); err == nil {
					freshest = max(freshest, time.Since(st.ModTime()))
				}
			}
			holderDone.Store(true)
			return nil
		})
	}()
	<-started
	mine, err := acquire(lock)
	if err != nil || mine == nil {
		t.Fatalf("waiter: %q %v", mine, err)
	}
	if !holderDone.Load() {
		t.Error("the waiter took over the lock while its holder was still running")
	}
	release(lock, mine)
	if err := <-errc; err != nil {
		t.Fatal(err)
	}
	if freshest >= 120*time.Millisecond {
		t.Errorf("the lock aged %v while held (stale after 120ms)", freshest)
	}
	gone(t, lock)
}

// TestWithBuildLockTakesOverAStaleLock: the lock of a holder that died (no
// heartbeat, older than lockStale) is taken over at once.
func TestWithBuildLockTakesOverAStaleLock(t *testing.T) {
	lockTiming(t, 5*time.Millisecond, 30*time.Second, 100*time.Millisecond)
	root, lock := lockRoot(t)
	if err := os.WriteFile(lock, []byte("1 dead\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-time.Second)
	if err := os.Chtimes(lock, old, old); err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	ran := false
	if err := WithBuildLock(root, func() error { ran = true; return nil }); err != nil || !ran {
		t.Fatalf("err %v ran %v", err, ran)
	}
	if time.Since(start) > 5*time.Second {
		t.Errorf("took %v to take over a stale lock", time.Since(start))
	}
	gone(t, lock)
}

// TestWithBuildLockBusy: a live lock still held after lockWait is a
// *LockBusyError; fn does not run and the other lock stays.
func TestWithBuildLockBusy(t *testing.T) {
	lockTiming(t, 5*time.Millisecond, 30*time.Millisecond, time.Hour)
	root, lock := lockRoot(t)
	if err := os.WriteFile(lock, []byte("1 other\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	ran := false
	err := WithBuildLock(root, func() error { ran = true; return nil })
	var busy *LockBusyError
	if !errors.As(err, &busy) || ran || busy.Holder != "1 other" || busy.Lock != lock {
		t.Errorf("err %v ran %v", err, ran)
	}
	if got, err := os.ReadFile(lock); err != nil || string(got) != "1 other\n" {
		t.Errorf("the other process's lock changed: %q %v", got, err)
	}
}

// TestHeartbeatNeverTouchesAnotherHoldersLock: once the lock holds another
// content the heartbeat leaves its mtime alone and ends; stop still joins.
func TestHeartbeatNeverTouchesAnotherHoldersLock(t *testing.T) {
	_, lock := lockRoot(t)
	if err := os.WriteFile(lock, []byte("mine\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	stop := heartbeat(lock, []byte("mine\n"), 5*time.Millisecond)
	if err := os.WriteFile(lock, []byte("next holder\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-time.Hour).Truncate(time.Second)
	if err := os.Chtimes(lock, old, old); err != nil {
		t.Fatal(err)
	}
	time.Sleep(60 * time.Millisecond)
	stop()
	stop()
	if st, err := os.Stat(lock); err != nil || !st.ModTime().Equal(old) {
		t.Errorf("the next holder's lock was touched: %v %v", st.ModTime(), err)
	}
}

// TestSearchWaitsForWithBuildLock: a search that must build the full-text
// index waits for a rebuild holding the lock, then builds it.
func TestSearchWaitsForWithBuildLock(t *testing.T) {
	lockTiming(t, 5*time.Millisecond, time.Minute, time.Hour)
	root := noIndexCheckout(t)
	started := make(chan struct{})
	var holderDone atomic.Bool
	errc := make(chan error, 1)
	go func() {
		errc <- WithBuildLock(root, func() error {
			close(started)
			time.Sleep(80 * time.Millisecond)
			holderDone.Store(true)
			return nil
		})
	}()
	<-started
	var errb bytes.Buffer
	_, rebuilt, err := ensure(root, &errb)
	if err != nil || !rebuilt || !holderDone.Load() || errb.String() != rebuildLines(root) {
		t.Errorf("rebuilt %v err %v holder done %v stderr %q", rebuilt, err, holderDone.Load(), errb.String())
	}
	if err := <-errc; err != nil {
		t.Fatal(err)
	}
}
