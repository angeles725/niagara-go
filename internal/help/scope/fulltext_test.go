package scope

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/angeles725/niagara-go/internal/help/fts"
	"github.com/angeles725/niagara-go/internal/help/fts/ftsfixture"
	ftbuild "github.com/angeles725/niagara-go/internal/helpbuild/fulltext"
)

// lockTiming sets the lock timing for one test.
func lockTiming(t *testing.T, poll, wait, stale time.Duration) {
	t.Helper()
	p, w, s := lockPoll, lockWait, lockStale
	t.Cleanup(func() { lockPoll, lockWait, lockStale = p, w, s })
	lockPoll, lockWait, lockStale = poll, wait, stale
}

// noIndexCheckout is the full-text fixture checkout without indexes, with
// its own empty cache directory.
func noIndexCheckout(t *testing.T) string {
	t.Helper()
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	return ftsfixture.Checkout(t, abs(t, helpTestdata), true)
}

func rebuildLines(root string) string {
	return "building the full-text index for " + root + " (one-time, after any corpus change)...\n" +
		"full-text index: 20 documents -> " + filepath.Join(root, "indexes", "fulltext.sqlite") + "\n"
}

func TestEnsureFulltextBuildsOnceThenReuses(t *testing.T) {
	root := noIndexCheckout(t)
	var errb bytes.Buffer
	rebuilt, err := EnsureFulltext(root, &errb)
	if err != nil || !rebuilt || errb.String() != rebuildLines(root) {
		t.Fatalf("first: rebuilt %v err %v stderr %q", rebuilt, err, errb.String())
	}
	errb.Reset()
	if rebuilt, err := EnsureFulltext(root, &errb); err != nil || rebuilt || errb.Len() != 0 {
		t.Errorf("second: rebuilt %v err %v stderr %q", rebuilt, err, errb.String())
	}
	if _, err := os.Stat(fts.DBPath(root) + ".lock"); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("lock left behind: %v", err)
	}
	x, err := OpenFulltext(&errb)(root, "1.0")
	if err != nil {
		t.Fatal(err)
	}
	x.Close()
}

// TestEnsureFulltextConcurrentCallersBuildOnce: two callers racing on a
// missing index build it once; the other waits and uses it, silently.
func TestEnsureFulltextConcurrentCallersBuildOnce(t *testing.T) {
	lockTiming(t, 5*time.Millisecond, time.Minute, time.Hour)
	root := noIndexCheckout(t)
	var wg sync.WaitGroup
	var bufs [2]bytes.Buffer
	var rebuilt [2]bool
	var errs [2]error
	for i := range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			rebuilt[i], errs[i] = EnsureFulltext(root, &bufs[i])
		}()
	}
	wg.Wait()
	if errs[0] != nil || errs[1] != nil || rebuilt[0] == rebuilt[1] {
		t.Fatalf("rebuilt %v errs %v", rebuilt, errs)
	}
	builder, waiter := 0, 1
	if rebuilt[1] {
		builder, waiter = 1, 0
	}
	if bufs[builder].String() != rebuildLines(root) || bufs[waiter].Len() != 0 {
		t.Errorf("builder stderr %q, waiter stderr %q", bufs[builder].String(), bufs[waiter].String())
	}
}

// TestEnsureFulltextWaitsForTheLockHolder: a live lock (another process
// building) is waited for; the index it writes is used without a rebuild.
func TestEnsureFulltextWaitsForTheLockHolder(t *testing.T) {
	lockTiming(t, 5*time.Millisecond, time.Minute, time.Hour)
	root := noIndexCheckout(t)
	lock := fts.DBPath(root) + ".lock"
	if err := os.WriteFile(lock, []byte("1 other\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		time.Sleep(50 * time.Millisecond)
		_, err := ftbuild.Build(root, ftbuild.Options{})
		if rerr := os.Remove(lock); err == nil {
			err = rerr
		}
		done <- err
	}()
	start := time.Now()
	var errb bytes.Buffer
	rebuilt, err := EnsureFulltext(root, &errb)
	if berr := <-done; berr != nil {
		t.Fatal(berr)
	}
	if err != nil || rebuilt || errb.Len() != 0 || time.Since(start) < 50*time.Millisecond {
		t.Errorf("rebuilt %v err %v stderr %q after %v", rebuilt, err, errb.String(), time.Since(start))
	}
}

// TestEnsureFulltextTakesOverAStaleLock: a lock older than lockStale (a
// process that died while building) is replaced at once and removed after.
func TestEnsureFulltextTakesOverAStaleLock(t *testing.T) {
	lockTiming(t, 5*time.Millisecond, time.Minute, 10*time.Minute)
	root := noIndexCheckout(t)
	lock := fts.DBPath(root) + ".lock"
	if err := os.WriteFile(lock, []byte("1 dead\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-11 * time.Minute)
	if err := os.Chtimes(lock, old, old); err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	var errb bytes.Buffer
	rebuilt, err := EnsureFulltext(root, &errb)
	if err != nil || !rebuilt || errb.String() != rebuildLines(root) || time.Since(start) > 30*time.Second {
		t.Errorf("rebuilt %v err %v stderr %q after %v", rebuilt, err, errb.String(), time.Since(start))
	}
	if _, err := os.Stat(lock); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("stale lock not removed: %v", err)
	}
}

// TestEnsureFulltextBuildsAfterTheWait: a live lock still held after
// lockWait does not block the search: it builds anyway (atomically) and
// leaves the other process's lock alone.
func TestEnsureFulltextBuildsAfterTheWait(t *testing.T) {
	lockTiming(t, 5*time.Millisecond, 30*time.Millisecond, time.Hour)
	root := noIndexCheckout(t)
	lock := fts.DBPath(root) + ".lock"
	if err := os.WriteFile(lock, []byte("1 slow\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var errb bytes.Buffer
	rebuilt, err := EnsureFulltext(root, &errb)
	if err != nil || !rebuilt || errb.String() != rebuildLines(root) {
		t.Errorf("rebuilt %v err %v stderr %q", rebuilt, err, errb.String())
	}
	if _, err := os.Stat(lock); err != nil {
		t.Errorf("the other process's lock was removed: %v", err)
	}
}

// TestEnsureFulltextUnwritableFailsOnce: with no writable location the error
// is *UnwritableIndexError; a second call of the process returns it at once
// without repeating the notes.
func TestEnsureFulltextUnwritableFailsOnce(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("read-only directories need a non-root POSIX user")
	}
	root := noIndexCheckout(t)
	cache := filepath.Join(t.TempDir(), "ro")
	t.Setenv("XDG_CACHE_HOME", cache)
	for _, d := range []string{cache, filepath.Join(root, "indexes")} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(d, 0o500); err != nil {
			t.Fatal(err)
		}
		defer os.Chmod(d, 0o755)
	}
	var errb bytes.Buffer
	_, err := EnsureFulltext(root, &errb)
	var ue *UnwritableIndexError
	if !errors.As(err, &ue) || !ue.Missing || errb.Len() == 0 {
		t.Fatalf("err %v stderr %q", err, errb.String())
	}
	errb.Reset()
	if _, err2 := EnsureFulltext(root, &errb); err2 != err || errb.Len() != 0 {
		t.Errorf("second call: err %v stderr %q", err2, errb.String())
	}
}

func TestPyReasonSpellsPythonsExceptions(t *testing.T) {
	for _, c := range []struct {
		err  error
		want string
	}{
		{&fs.PathError{Op: "mkdir", Path: "/c/niagara-help", Err: syscall.EACCES},
			fmt.Sprintf("[Errno %d] Permission denied: '/c/niagara-help'", int(syscall.EACCES))},
		{&fs.PathError{Op: "mkdir", Path: "/it's", Err: syscall.EACCES},
			fmt.Sprintf(`[Errno %d] Permission denied: "/it's"`, int(syscall.EACCES))},
		{&fs.PathError{Op: "open", Path: "/x/fulltext.sqlite.1.tmp", Err: syscall.EACCES},
			"unable to open database file"},
		{errors.New("disk I/O error"), "disk I/O error"},
	} {
		if got := pyReason(c.err); got != c.want {
			t.Errorf("pyReason(%v) = %q, want %q", c.err, got, c.want)
		}
	}
}
