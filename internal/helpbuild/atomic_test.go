package helpbuild

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"

	"github.com/angeles725/niagara-go/internal/pyjson"
)

func listDir(t *testing.T, dir string) []string {
	t.Helper()
	ents, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range ents {
		names = append(names, e.Name())
	}
	sort.Strings(names)
	return names
}

func readText(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// failRename makes the n-th call of the package rename hook (1-based, 0 =
// every call) fail, for the rest of the test.
func failRename(t *testing.T, n int) error {
	t.Helper()
	errInjected := errors.New("injected rename failure")
	calls := 0
	real := rename
	rename = func(from, to string) error {
		calls++
		if n == 0 || calls == n {
			return errInjected
		}
		return real(from, to)
	}
	t.Cleanup(func() { rename = real })
	return errInjected
}

func okObject() pyjson.Value {
	v := pyjson.NewObject()
	v.Set("ok", pyjson.IntValue(1))
	return v
}

// test_atomic_writes.TestAtomicWrite.setUp: write_json(path, {"ok": 1}).
func atomicFixture(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "indexes", "x.json")
	if err := WriteJSON(path, okObject(), DumpDefault); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestWriteJSONCreatesParentsWithPythonDefaultDump(t *testing.T) {
	path := atomicFixture(t)
	if got := readText(t, path); got != `{"ok": 1}` {
		t.Fatalf("content %q, want {\"ok\": 1}", got)
	}
}

// test_failed_write_keeps_the_previous_file_and_no_temp: Python fails inside
// json.dumps; a Go Value always encodes, so the failure is injected where a
// Go write can fail (the rename that swaps the temp file in).
func TestFailedWriteKeepsThePreviousFileAndNoTemp(t *testing.T) {
	path := atomicFixture(t)
	injected := failRename(t, 0)
	err := WriteBytes(path, []byte(`{"bad": 1}`))
	if !errors.Is(err, injected) {
		t.Fatalf("err %v, want the rename failure", err)
	}
	if got := readText(t, path); got != `{"ok": 1}` {
		t.Fatalf("previous file changed: %q", got)
	}
	if got := listDir(t, filepath.Dir(path)); strings.Join(got, ",") != "x.json" {
		t.Fatalf("directory holds %q, want only x.json", got)
	}
}

// A real failure (the target is a non-empty directory) also leaves no temp.
func TestFailedWriteOverADirectoryLeavesNoTemp(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "x.json")
	writeFile(t, filepath.Join(target, "inner"), []byte("x"))
	if err := WriteText(target, "data"); err == nil {
		t.Fatal("writing over a non-empty directory succeeded")
	}
	if got := listDir(t, dir); strings.Join(got, ",") != "x.json" {
		t.Fatalf("directory holds %q, want only x.json", got)
	}
}

// test_written_file_is_world_readable.
func TestWrittenFileIsWorldReadable(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows has no POSIX mode bits")
	}
	st, err := os.Stat(atomicFixture(t))
	if err != nil {
		t.Fatal(err)
	}
	if st.Mode().Perm() != 0o644 {
		t.Fatalf("mode %o, want 644", st.Mode().Perm())
	}
}

func TestWriteBytesReplacesAnExistingFile(t *testing.T) {
	path := atomicFixture(t)
	if err := WriteBytes(path, []byte("second")); err != nil {
		t.Fatal(err)
	}
	if got := readText(t, path); got != "second" {
		t.Fatalf("content %q", got)
	}
	if got := listDir(t, filepath.Dir(path)); strings.Join(got, ",") != "x.json" {
		t.Fatalf("directory holds %q", got)
	}
}

// rebuiltFixture is test_atomic_writes.TestCleanTreesSurviveAFailedRebuild's
// previous build: <root>/guides-clean/G/old.txt.
func rebuiltFixture(t *testing.T) (root, final string) {
	t.Helper()
	root = t.TempDir()
	final = filepath.Join(root, "guides-clean")
	writeFile(t, filepath.Join(final, "G", "old.txt"), []byte("previous build"))
	return root, final
}

func treeFiles(t *testing.T, dir string) []string {
	t.Helper()
	var out []string
	err := filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			rel, _ := filepath.Rel(dir, p)
			out = append(out, filepath.ToSlash(rel))
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(out)
	return out
}

func noDotEntries(t *testing.T, root string) {
	t.Helper()
	for _, n := range listDir(t, root) {
		if strings.HasPrefix(n, ".") {
			t.Fatalf("stray temp entry %s left in %s", n, root)
		}
	}
}

// buildNew writes the new tree without WriteBytes, so the rename hook
// counts only RebuiltDir's own renames (1 = final aside, 2 = the swap,
// 3 = the restore).
func buildNew(tmp string) error {
	if err := os.MkdirAll(filepath.Join(tmp, "G"), 0o755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(tmp, "G", "new.txt"), []byte("new build"), 0o644)
}

func TestRebuiltDirSwapsTheNewTreeIn(t *testing.T) {
	root, final := rebuiltFixture(t)
	if err := RebuiltDir(final, buildNew); err != nil {
		t.Fatal(err)
	}
	if got := treeFiles(t, final); strings.Join(got, ",") != "G/new.txt" {
		t.Fatalf("tree %q, want only G/new.txt", got)
	}
	noDotEntries(t, root)
	if runtime.GOOS != "windows" {
		st, _ := os.Stat(final)
		if st.Mode().Perm() != 0o755 {
			t.Fatalf("mode %o, want 755", st.Mode().Perm())
		}
	}
}

func TestRebuiltDirFirstBuildCreatesTheTree(t *testing.T) {
	final := filepath.Join(t.TempDir(), "a", "devguide-clean")
	if err := RebuiltDir(final, buildNew); err != nil {
		t.Fatal(err)
	}
	if got := treeFiles(t, final); strings.Join(got, ",") != "G/new.txt" {
		t.Fatalf("tree %q", got)
	}
}

// test_devguide_clean / test_guides_clean: a run that dies keeps the old
// tree and leaves no dot entry; the next run replaces it.
func TestRebuiltDirFailedBuildKeepsTheOldTree(t *testing.T) {
	root, final := rebuiltFixture(t)
	errStop := errors.New("interrupted")
	err := RebuiltDir(final, func(tmp string) error {
		if err := buildNew(tmp); err != nil {
			return err
		}
		return errStop
	})
	if !errors.Is(err, errStop) {
		t.Fatalf("err %v, want the build error", err)
	}
	if got := treeFiles(t, final); strings.Join(got, ",") != "G/old.txt" {
		t.Fatalf("tree %q, want the old tree", got)
	}
	noDotEntries(t, root)
	if err := RebuiltDir(final, buildNew); err != nil {
		t.Fatal(err)
	}
	if got := treeFiles(t, final); strings.Join(got, ",") != "G/new.txt" {
		t.Fatalf("tree %q after the second run", got)
	}
}

// Python catches BaseException (KeyboardInterrupt too): a panic cleans up.
func TestRebuiltDirPanicCleansUpAndRepanics(t *testing.T) {
	root, final := rebuiltFixture(t)
	func() {
		defer func() {
			if recover() == nil {
				t.Fatal("panic swallowed")
			}
		}()
		_ = RebuiltDir(final, func(tmp string) error {
			_ = buildNew(tmp)
			panic("boom")
		})
	}()
	if got := treeFiles(t, final); strings.Join(got, ",") != "G/old.txt" {
		t.Fatalf("tree %q, want the old tree", got)
	}
	noDotEntries(t, root)
}

// The swap fails: the old tree is renamed back, the temp tree removed.
func TestRebuiltDirFailedSwapRestoresTheOldTree(t *testing.T) {
	root, final := rebuiltFixture(t)
	injected := failRename(t, 2)
	err := RebuiltDir(final, buildNew)
	if !errors.Is(err, injected) {
		t.Fatalf("err %v, want the swap failure", err)
	}
	var re *RestoreError
	if errors.As(err, &re) {
		t.Fatalf("restore succeeded but got %v", err)
	}
	if got := treeFiles(t, final); strings.Join(got, ",") != "G/old.txt" {
		t.Fatalf("tree %q, want the old tree", got)
	}
	noDotEntries(t, root)
}

// Swap and restore both fail: one error names both and where the old tree
// was kept (Python chains it from the swap error).
func TestRebuiltDirFailedRestoreNamesWhereTheOldTreeIs(t *testing.T) {
	_, final := rebuiltFixture(t)
	real := rename
	calls := 0
	swapErr, restoreErr := errors.New("swap failed"), errors.New("restore failed")
	rename = func(from, to string) error {
		calls++
		switch calls {
		case 2:
			return swapErr
		case 3:
			return restoreErr
		}
		return real(from, to)
	}
	t.Cleanup(func() { rename = real })
	err := RebuiltDir(final, buildNew)
	var re *RestoreError
	if !errors.As(err, &re) || !errors.Is(err, swapErr) {
		t.Fatalf("err %v, want a RestoreError wrapping the swap error", err)
	}
	want := "swapping in the rebuilt " + final + " failed (swap failed); restoring the " +
		"previous tree failed too (restore failed): it is kept at " + re.Kept
	if err.Error() != want {
		t.Fatalf("message\n got %s\nwant %s", err.Error(), want)
	}
	if got := treeFiles(t, re.Kept); strings.Join(got, ",") != "G/old.txt" {
		t.Fatalf("kept tree %q", got)
	}
}

// Renaming the old tree aside fails (the first rename): the error comes
// back, the old tree is untouched and the temp tree is removed.
func TestRebuiltDirFailedRenameAsideKeepsTheOldTree(t *testing.T) {
	root, final := rebuiltFixture(t)
	injected := failRename(t, 1)
	if err := RebuiltDir(final, buildNew); !errors.Is(err, injected) {
		t.Fatalf("err %v, want the rename-aside failure", err)
	}
	if got := treeFiles(t, final); strings.Join(got, ",") != "G/old.txt" {
		t.Fatalf("tree %q, want the old tree", got)
	}
	noDotEntries(t, root)
}

// Removing the old tree after a good swap fails: like atomic.py's
// shutil.rmtree(old, ignore_errors=True) the rebuild still succeeds, the new
// tree is in place and the aside is left behind.
func TestRebuiltDirIgnoresAFailedRemovalOfTheOldTree(t *testing.T) {
	root, final := rebuiltFixture(t)
	real := removeOld
	var removed []string
	removeOld = func(p string) error {
		removed = append(removed, p)
		return errors.New("injected removal failure")
	}
	t.Cleanup(func() { removeOld = real })
	if err := RebuiltDir(final, buildNew); err != nil {
		t.Fatalf("err %v, want nil (the removal error is ignored)", err)
	}
	if got := treeFiles(t, final); strings.Join(got, ",") != "G/new.txt" {
		t.Fatalf("tree %q, want the new tree", got)
	}
	if len(removed) != 1 || !strings.Contains(filepath.Base(removed[0]), ".old-") {
		t.Fatalf("removed %q, want the one aside tree", removed)
	}
	if got := treeFiles(t, removed[0]); strings.Join(got, ",") != "G/old.txt" {
		t.Fatalf("aside %q, want the old tree left behind", got)
	}
	_ = os.RemoveAll(removed[0])
	noDotEntries(t, root)
}

func writeFile(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}
