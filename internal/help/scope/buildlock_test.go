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

// manualTicks replaces the heartbeat's ticker for one test: each value
// sent on the returned channel is one tick.
func manualTicks(t *testing.T) chan time.Time {
	t.Helper()
	ticks := make(chan time.Time)
	saved := heartbeatTicks
	t.Cleanup(func() { heartbeatTicks = saved })
	heartbeatTicks = func(time.Duration) (<-chan time.Time, func()) { return ticks, func() {} }
	return ticks
}

// tick sends two ticks: the second send completes only once the goroutine
// has finished the first one (the channel is unbuffered).
func tick(ticks chan time.Time) {
	ticks <- time.Now()
	ticks <- time.Now()
}

func lockAge(t *testing.T, lock string) time.Duration {
	t.Helper()
	st, err := os.Stat(lock)
	if err != nil {
		t.Fatal(err)
	}
	return time.Since(st.ModTime())
}

// TestWithBuildLockHeartbeatKeepsALongHolder: a holder whose lock has aged
// past lockStale (simulated by backdating its mtime) is kept fresh by one
// heartbeat tick, so a waiter does not take it over and gets the lock only
// after fn has returned. The ticks are driven by hand: no wall-clock margin.
func TestWithBuildLockHeartbeatKeepsALongHolder(t *testing.T) {
	lockTiming(t, time.Millisecond, time.Minute, time.Hour)
	ticks := manualTicks(t)
	root, lock := lockRoot(t)
	started, proceed := make(chan struct{}), make(chan struct{})
	var holderDone atomic.Bool
	errc := make(chan error, 1)
	go func() {
		errc <- WithBuildLock(root, func() error {
			close(started)
			<-proceed
			holderDone.Store(true)
			return nil
		})
	}()
	<-started
	old := time.Now().Add(-2 * time.Hour)
	if err := os.Chtimes(lock, old, old); err != nil {
		t.Fatal(err)
	}
	tick(ticks)
	if age := lockAge(t, lock); age >= time.Minute {
		t.Fatalf("the heartbeat did not refresh the lock: aged %v", age)
	}
	got := make(chan []byte, 1)
	go func() {
		mine, err := acquire(lock)
		if err != nil {
			t.Error(err)
		}
		got <- mine
	}()
	select {
	case <-got:
		t.Fatal("the waiter took over the lock while its holder was still running")
	case <-time.After(50 * time.Millisecond):
	}
	close(proceed)
	mine := <-got
	if mine == nil || !holderDone.Load() {
		t.Errorf("waiter got %q, holder done %v", mine, holderDone.Load())
	}
	release(lock, mine)
	if err := <-errc; err != nil {
		t.Fatal(err)
	}
	gone(t, lock)
}

func TestHeartbeatEvery(t *testing.T) {
	lockTiming(t, time.Millisecond, 2*time.Minute, 10*time.Minute)
	if got := heartbeatEvery(); got != 30*time.Second {
		t.Errorf("heartbeat every %v with the default timing, want lockWait/4 = 30s", got)
	}
	lockTiming(t, time.Millisecond, time.Minute, 20*time.Second)
	if got := heartbeatEvery(); got != 5*time.Second {
		t.Errorf("heartbeat every %v with lockStale < lockWait, want lockStale/4", got)
	}
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

// TestHeartbeatNeverTouchesAnotherHoldersLock: a lock that holds another
// process's content is left alone and the heartbeat goroutine ends; stop
// still joins. The other content is in place before the first tick.
func TestHeartbeatNeverTouchesAnotherHoldersLock(t *testing.T) {
	_, lock := lockRoot(t)
	if err := os.WriteFile(lock, []byte("next holder\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-time.Hour).Truncate(time.Second)
	if err := os.Chtimes(lock, old, old); err != nil {
		t.Fatal(err)
	}
	ticks := make(chan time.Time)
	stop, ended := heartbeat(lock, []byte("mine\n"), ticks)
	ticks <- time.Now()
	select {
	case <-ended:
	case <-time.After(time.Minute):
		t.Fatal("the heartbeat goroutine did not end on another holder's lock")
	}
	stop()
	stop()
	if st, err := os.Stat(lock); err != nil || !st.ModTime().Equal(old) {
		t.Errorf("the other holder's lock was touched: %v %v", st.ModTime(), err)
	}
}

// TestHeartbeatTouchesItsOwnLock: each tick refreshes the mtime of a lock
// that holds this process's content; stop ends the goroutine.
func TestHeartbeatTouchesItsOwnLock(t *testing.T) {
	_, lock := lockRoot(t)
	if err := os.WriteFile(lock, []byte("mine\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-time.Hour)
	if err := os.Chtimes(lock, old, old); err != nil {
		t.Fatal(err)
	}
	ticks := make(chan time.Time)
	stop, ended := heartbeat(lock, []byte("mine\n"), ticks)
	tick(ticks)
	if age := lockAge(t, lock); age >= time.Minute {
		t.Errorf("own lock not refreshed: aged %v", age)
	}
	stop()
	select {
	case <-ended:
	default:
		t.Error("stop returned before the goroutine ended")
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
