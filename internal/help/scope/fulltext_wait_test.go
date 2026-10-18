package scope

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/angeles725/niagara-go/internal/help/fts"
)

// The timings are wide on purpose: lockWait 100 ms against a 20 ms
// heartbeat leaves five touches per wait on a loaded machine.
const (
	testPoll      = 5 * time.Millisecond
	testWait      = 100 * time.Millisecond
	testHeartbeat = 20 * time.Millisecond
)

// followMax sets lockFollowMax for one test.
func followMax(t *testing.T, d time.Duration) {
	t.Helper()
	old := lockFollowMax
	t.Cleanup(func() { lockFollowMax = old })
	lockFollowMax = d
}

// holder is another process's lock: its content, and the time of its last
// heartbeat touch.
type holder struct {
	mu        sync.Mutex
	lastTouch time.Time
	done      chan struct{}
}

func (h *holder) last() time.Time {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.lastTouch
}

// heartbeatHolder writes lock with content and touches its mtime every
// testHeartbeat for d, then releases it when release is true. A failed
// touch fails the test.
func heartbeatHolder(t *testing.T, lock, content string, d time.Duration, release bool) *holder {
	t.Helper()
	if err := os.WriteFile(lock, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	h := &holder{lastTouch: time.Now(), done: make(chan struct{})}
	go func() {
		defer close(h.done)
		end := time.Now().Add(d)
		for time.Now().Before(end) {
			time.Sleep(testHeartbeat)
			now := time.Now()
			if err := os.Chtimes(lock, now, now); err != nil {
				t.Errorf("heartbeat: %v", err)
				return
			}
			h.mu.Lock()
			h.lastTouch = now
			h.mu.Unlock()
		}
		if release {
			os.Remove(lock)
		}
	}()
	return h
}

// TestEnsureFulltextWaitsForAHeartbeatingRebuild: a holder that keeps
// touching its lock (nh rebuild-all's heartbeat) is still rewriting the
// corpus, so a search waits past lockWait until it releases, with one note,
// and then builds; it never indexes the half-rewritten corpus.
func TestEnsureFulltextWaitsForAHeartbeatingRebuild(t *testing.T) {
	lockTiming(t, testPoll, testWait, time.Hour)
	root := noIndexCheckout(t)
	lock := fts.DBPath(root) + ".lock"
	hold := 600 * time.Millisecond
	h := heartbeatHolder(t, lock, "1 rebuild\n", hold, true)
	start := time.Now()
	var errb bytes.Buffer
	_, rebuilt, err := ensure(root, &errb)
	elapsed := time.Since(start)
	<-h.done
	if err != nil || !rebuilt {
		t.Fatalf("rebuilt %v err %v", rebuilt, err)
	}
	if elapsed < hold {
		t.Errorf("built after %v, while the rebuild still held the lock (%v)", elapsed, hold)
	}
	if want := waitingNote(lock, []byte("1 rebuild\n")) + rebuildLines(root); errb.String() != want {
		t.Errorf("stderr %q, want %q", errb.String(), want)
	}
}

// TestEnsureFulltextBuildsWhenTheHeartbeatStops: a holder whose lock stops
// moving (a hung or remote process, its pid not checkable) no longer counts
// as a running rebuild: a full lockWait after its last touch the search
// builds anyway, noting the wait once, and leaves the lock alone.
func TestEnsureFulltextBuildsWhenTheHeartbeatStops(t *testing.T) {
	lockTiming(t, testPoll, testWait, time.Hour)
	root := noIndexCheckout(t)
	lock := fts.DBPath(root) + ".lock"
	h := heartbeatHolder(t, lock, "1 rebuild\n", 250*time.Millisecond, false)
	var errb bytes.Buffer
	_, rebuilt, err := ensure(root, &errb)
	built := time.Now()
	<-h.done
	if err != nil || !rebuilt {
		t.Fatalf("rebuilt %v err %v", rebuilt, err)
	}
	if gap := built.Sub(h.last()); gap < testWait {
		t.Errorf("built %v after the last heartbeat, before a full lockWait (%v)", gap, testWait)
	}
	if want := waitingNote(lock, []byte("1 rebuild\n")) + rebuildLines(root); errb.String() != want {
		t.Errorf("stderr %q, want %q", errb.String(), want)
	}
	if _, err := os.Stat(lock); err != nil {
		t.Errorf("the other process's lock was removed: %v", err)
	}
}

// TestEnsureFulltextGivesANewHolderAFullWait: when the lock changes hands
// after lockWait, the new holder gets a full lockWait of its own instead of
// an immediate build.
func TestEnsureFulltextGivesANewHolderAFullWait(t *testing.T) {
	lockTiming(t, testPoll, testWait, time.Hour)
	root := noIndexCheckout(t)
	lock := fts.DBPath(root) + ".lock"
	h := heartbeatHolder(t, lock, "1 rebuild\n", 200*time.Millisecond, false)
	handover := make(chan time.Time, 1)
	go func() {
		<-h.done
		at := time.Now() // before the write: the search can only see the new holder after it
		if err := os.WriteFile(lock, []byte("2 other\n"), 0o644); err != nil {
			t.Errorf("handover: %v", err)
		}
		handover <- at
	}()
	var errb bytes.Buffer
	_, rebuilt, err := ensure(root, &errb)
	built := time.Now()
	at := <-handover
	if err != nil || !rebuilt {
		t.Fatalf("rebuilt %v err %v", rebuilt, err)
	}
	if gap := built.Sub(at); gap < testWait {
		t.Errorf("built %v after the new holder took the lock, before a full lockWait (%v)", gap, testWait)
	}
}

// TestEnsureFulltextFollowsARebuildAtMostLockFollowMax: a holder whose
// heartbeat never stops (its work hung) holds a search back only
// lockFollowMax; then the search builds anyway.
func TestEnsureFulltextFollowsARebuildAtMostLockFollowMax(t *testing.T) {
	lockTiming(t, testPoll, testWait, time.Hour)
	followMax(t, 300*time.Millisecond)
	root := noIndexCheckout(t)
	lock := fts.DBPath(root) + ".lock"
	h := heartbeatHolder(t, lock, "1 rebuild\n", 2*time.Second, false)
	start := time.Now()
	var errb bytes.Buffer
	_, rebuilt, err := ensure(root, &errb)
	elapsed := time.Since(start)
	<-h.done
	if err != nil || !rebuilt || !strings.HasSuffix(errb.String(), rebuildLines(root)) {
		t.Errorf("rebuilt %v err %v stderr %q", rebuilt, err, errb.String())
	}
	if elapsed < 300*time.Millisecond || elapsed > 1500*time.Millisecond {
		t.Errorf("built after %v, want about lockFollowMax (300ms)", elapsed)
	}
}

// TestEnsureFulltextCapIsNotResetByChangingHolders: lock content that keeps
// changing faster than lockWait (holders replacing each other, or a torn
// read of a lock being written) does not extend a search past lockFollowMax.
func TestEnsureFulltextCapIsNotResetByChangingHolders(t *testing.T) {
	lockTiming(t, testPoll, testWait, time.Hour)
	followMax(t, 300*time.Millisecond)
	root := noIndexCheckout(t)
	lock := fts.DBPath(root) + ".lock"
	if err := os.WriteFile(lock, []byte("1 a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	stop := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; ; i++ {
			select {
			case <-stop:
				return
			case <-time.After(testHeartbeat):
				os.WriteFile(lock, []byte(fmt.Sprintf("%d changing\n", i+2)), 0o644)
			}
		}
	}()
	start := time.Now()
	var errb bytes.Buffer
	_, rebuilt, err := ensure(root, &errb)
	elapsed := time.Since(start)
	close(stop)
	<-done
	if err != nil || !rebuilt {
		t.Errorf("rebuilt %v err %v", rebuilt, err)
	}
	if elapsed > 1500*time.Millisecond {
		t.Errorf("built after %v, want about lockFollowMax (300ms)", elapsed)
	}
}
