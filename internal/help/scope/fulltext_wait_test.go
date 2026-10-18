package scope

import (
	"bytes"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/angeles725/niagara-go/internal/help/fts"
)

// heartbeatHolder is another process holding lock (a live lock content) that
// touches its mtime every interval for d, then releases it when release is
// true; the returned channel closes when it is done.
func heartbeatHolder(t *testing.T, lock string, interval, d time.Duration, release bool) <-chan struct{} {
	t.Helper()
	if err := os.WriteFile(lock, []byte("1 rebuild\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		end := time.Now().Add(d)
		for time.Now().Before(end) {
			time.Sleep(interval)
			now := time.Now()
			os.Chtimes(lock, now, now)
		}
		if release {
			os.Remove(lock)
		}
	}()
	return done
}

// TestEnsureFulltextWaitsForAHeartbeatingRebuild: a holder that keeps
// touching its lock (nh rebuild-all's heartbeat) is still rewriting the
// corpus, so a search waits past lockWait until it releases, with one note,
// and then builds; it never indexes the half-rewritten corpus.
func TestEnsureFulltextWaitsForAHeartbeatingRebuild(t *testing.T) {
	lockTiming(t, 5*time.Millisecond, 60*time.Millisecond, time.Hour)
	root := noIndexCheckout(t)
	lock := fts.DBPath(root) + ".lock"
	hold := 400 * time.Millisecond
	done := heartbeatHolder(t, lock, 15*time.Millisecond, hold, true)
	start := time.Now()
	var errb bytes.Buffer
	_, rebuilt, err := ensure(root, &errb)
	elapsed := time.Since(start)
	<-done
	if err != nil || !rebuilt {
		t.Fatalf("rebuilt %v err %v", rebuilt, err)
	}
	if elapsed < hold {
		t.Errorf("built after %v, while the rebuild still held the lock (%v)", elapsed, hold)
	}
	note := waitingNote(lock, []byte("1 rebuild\n"))
	if errb.String() != note+rebuildLines(root) {
		t.Errorf("stderr %q, want the waiting note once, then the build", errb.String())
	}
}

// TestEnsureFulltextBuildsWhenTheHeartbeatStops: a holder whose lock stops
// moving (a hung or remote process, its pid not checkable) no longer counts
// as a running rebuild: lockWait after the last touch the search builds
// anyway and leaves the other process's lock alone.
func TestEnsureFulltextBuildsWhenTheHeartbeatStops(t *testing.T) {
	lockTiming(t, 5*time.Millisecond, 60*time.Millisecond, time.Hour)
	root := noIndexCheckout(t)
	lock := fts.DBPath(root) + ".lock"
	done := heartbeatHolder(t, lock, 15*time.Millisecond, 150*time.Millisecond, false)
	var errb bytes.Buffer
	_, rebuilt, err := ensure(root, &errb)
	<-done
	if err != nil || !rebuilt || !strings.HasSuffix(errb.String(), rebuildLines(root)) {
		t.Errorf("rebuilt %v err %v stderr %q", rebuilt, err, errb.String())
	}
	if _, err := os.Stat(lock); err != nil {
		t.Errorf("the other process's lock was removed: %v", err)
	}
}
