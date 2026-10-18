package scope

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
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
	_, rebuilt, err := ensure(root, &errb)
	if err != nil || !rebuilt || errb.String() != rebuildLines(root) {
		t.Fatalf("first: rebuilt %v err %v stderr %q", rebuilt, err, errb.String())
	}
	errb.Reset()
	if _, rebuilt, err := ensure(root, &errb); err != nil || rebuilt || errb.Len() != 0 {
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
			_, rebuilt[i], errs[i] = ensure(root, &bufs[i])
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
// The holder checks that ensure has not returned before it starts building:
// until then the index is missing and the lock held, so an early return can
// only mean ensure did not wait (no wall-clock bound that load can trip).
func TestEnsureFulltextWaitsForTheLockHolder(t *testing.T) {
	lockTiming(t, 5*time.Millisecond, time.Minute, time.Hour)
	root := noIndexCheckout(t)
	lock := fts.DBPath(root) + ".lock"
	if err := os.WriteFile(lock, []byte("1 other\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	returned := make(chan struct{})
	done := make(chan error, 1)
	start := time.Now() // before the holder's 50 ms, so a waiting ensure takes at least that
	go func() {
		time.Sleep(50 * time.Millisecond)
		select {
		case <-returned:
			done <- errors.New("ensure returned while the lock holder had not built the index yet")
			return
		default:
		}
		_, err := ftbuild.Build(root, ftbuild.Options{})
		if rerr := os.Remove(lock); err == nil {
			err = rerr
		}
		done <- err
	}()
	var errb bytes.Buffer
	_, rebuilt, err := ensure(root, &errb)
	elapsed := time.Since(start)
	close(returned)
	if berr := <-done; berr != nil {
		t.Fatal(berr)
	}
	if err != nil || rebuilt || errb.Len() != 0 || elapsed < 50*time.Millisecond {
		t.Errorf("rebuilt %v err %v stderr %q after %v", rebuilt, err, errb.String(), elapsed)
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
	_, rebuilt, err := ensure(root, &errb)
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
	_, rebuilt, err := ensure(root, &errb)
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
	_, _, err := ensure(root, &errb)
	var ue *UnwritableIndexError
	if !errors.As(err, &ue) || !ue.Missing || errb.Len() == 0 {
		t.Fatalf("err %v stderr %q", err, errb.String())
	}
	errb.Reset()
	if _, _, err2 := ensure(root, &errb); err2 != err || errb.Len() != 0 {
		t.Errorf("second call: err %v stderr %q", err2, errb.String())
	}
}

// deadPID is the pid of a process that has exited (and been waited for).
func deadPID(t *testing.T) int {
	t.Helper()
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(self, "-test.run=^$")
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}
	return cmd.Process.Pid
}

// lockOf is the content a process with pid on this host writes in its lock.
func lockOf(pid int) []byte {
	f := strings.Fields(string(lockContent()))
	f[0] = strconv.Itoa(pid)
	return []byte(strings.Join(f, " ") + "\n")
}

func TestPidAlive(t *testing.T) {
	if alive, known := pidAlive(os.Getpid()); !alive || !known {
		t.Errorf("own pid: alive %v known %v", alive, known)
	}
	if alive, known := pidAlive(deadPID(t)); alive || !known {
		t.Errorf("exited pid: alive %v known %v", alive, known)
	}
}

// TestEnsureFulltextTakesOverADeadHoldersLock: a fresh lock whose pid is
// not running (nh ended by Ctrl-C) is taken over at once, not after
// lockWait.
func TestEnsureFulltextTakesOverADeadHoldersLock(t *testing.T) {
	lockTiming(t, 5*time.Millisecond, time.Minute, time.Hour)
	root := noIndexCheckout(t)
	lock := fts.DBPath(root) + ".lock"
	if err := os.WriteFile(lock, lockOf(deadPID(t)), 0o644); err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	var errb bytes.Buffer
	_, rebuilt, err := ensure(root, &errb)
	if err != nil || !rebuilt || errb.String() != rebuildLines(root) || time.Since(start) > 30*time.Second {
		t.Errorf("rebuilt %v err %v stderr %q after %v", rebuilt, err, errb.String(), time.Since(start))
	}
	if _, err := os.Stat(lock); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("dead holder's lock not removed: %v", err)
	}
}

// TestLiveHoldersLockIsNotStale: a lock naming a running process of this
// host, or written in a format without host, is waited for.
func TestLiveHoldersLockIsNotStale(t *testing.T) {
	lockTiming(t, time.Millisecond, time.Minute, time.Hour)
	now := time.Now()
	for _, content := range [][]byte{lockOf(os.Getppid()), []byte("1 slow\n"), nil,
		[]byte(fmt.Sprintf("%d other-os host t\n", deadPID(t)))} {
		if stale(content, now) {
			t.Errorf("stale(%q) = true", content)
		}
	}
	if !stale([]byte("1 slow\n"), now.Add(-2*time.Hour)) {
		t.Error("a lock older than lockStale is not stale")
	}
}

// TestTakeOverIsAtomic: of several waiters taking over the same stale lock
// at most one succeeds, and a waiter whose view is outdated (the lock was
// replaced by a fresh one) leaves the fresh lock alone; an abandoned guard
// is cleared.
func TestTakeOverIsAtomic(t *testing.T) {
	for range 20 {
		lock := filepath.Join(t.TempDir(), "fulltext.sqlite.lock")
		seen := lockOf(deadPID(t))
		if err := os.WriteFile(lock, seen, 0o644); err != nil {
			t.Fatal(err)
		}
		var wg sync.WaitGroup
		var mu sync.Mutex
		won := 0
		for range 4 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				if takeOver(lock, seen) {
					mu.Lock()
					won++
					mu.Unlock()
				}
			}()
		}
		wg.Wait()
		_, err := os.Stat(lock)
		if won > 1 || (won == 1) != errors.Is(err, fs.ErrNotExist) {
			t.Fatalf("%d waiters took the lock over (lock: %v)", won, err)
		}
		if won == 0 && !takeOver(lock, seen) {
			t.Fatal("the lock cannot be taken over once the waiters are done")
		}
		if ents, _ := os.ReadDir(filepath.Dir(lock)); len(ents) != 0 {
			t.Fatalf("left behind: %v", ents)
		}
	}
	lock := filepath.Join(t.TempDir(), "fulltext.sqlite.lock")
	fresh := lockContent()
	if err := os.WriteFile(lock, fresh, 0o644); err != nil {
		t.Fatal(err)
	}
	if takeOver(lock, []byte("1 linux host old\n")) {
		t.Fatal("took over a lock it did not see")
	}
	if got, err := os.ReadFile(lock); err != nil || !bytes.Equal(got, fresh) {
		t.Errorf("fresh lock changed: %q %v", got, err)
	}
	guard := lock + ".takeover"
	if err := os.WriteFile(guard, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if takeOver(lock, fresh) {
		t.Error("took over while another waiter holds the guard")
	}
	old := time.Now().Add(-2 * guardStale)
	if err := os.Chtimes(guard, old, old); err != nil {
		t.Fatal(err)
	}
	if takeOver(lock, fresh) || !takeOver(lock, fresh) {
		t.Error("an abandoned guard is not cleared")
	}
}

// TestEnsureFulltextStaleLockTwoWaitersBuildOnce: two searches meeting the
// same stale lock build the index once.
func TestEnsureFulltextStaleLockTwoWaitersBuildOnce(t *testing.T) {
	lockTiming(t, time.Millisecond, time.Minute, time.Hour)
	for range 5 {
		root := noIndexCheckout(t)
		if err := os.WriteFile(fts.DBPath(root)+".lock", lockOf(deadPID(t)), 0o644); err != nil {
			t.Fatal(err)
		}
		var wg sync.WaitGroup
		var rebuilt [2]bool
		var errs [2]error
		for i := range 2 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_, rebuilt[i], errs[i] = ensure(root, io.Discard)
			}()
		}
		wg.Wait()
		if errs[0] != nil || errs[1] != nil || rebuilt[0] == rebuilt[1] {
			t.Fatalf("rebuilt %v errs %v", rebuilt, errs)
		}
	}
}

// TestReleaseKeepsAnotherProcessesLock: a lock replaced after a takeover is
// not removed by the process that first held it.
func TestReleaseKeepsAnotherProcessesLock(t *testing.T) {
	lock := filepath.Join(t.TempDir(), "fulltext.sqlite.lock")
	if err := os.WriteFile(lock, []byte("other\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	release(lock, []byte("mine\n"))
	if _, err := os.Stat(lock); err != nil {
		t.Errorf("another process's lock removed: %v", err)
	}
	release(lock, []byte("other\n"))
	if _, err := os.Stat(lock); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("own lock kept: %v", err)
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
