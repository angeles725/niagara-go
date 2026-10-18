// Package atomicfile writes files and directory trees so that an interrupted writer never
// leaves a half-written result behind (the Go side of niagara_help_lib/atomic.py).
//
// A file write goes to a temp file in the same directory (".tmp-*<last 40 code points of
// the name>"), then a rename swaps it in; on error or panic the temp file is removed and the
// previous file is untouched. A rebuilt tree is built in an empty sibling temp directory
// and swapped in the same way, the old tree renamed aside and restored when the swap fails.
//
// Mode rules: a written file ends with mode 0644 and a rebuilt tree's root with 0755 (the
// explicit chmod calls of atomic.py; CreateTemp/MkdirTemp would leave 0600/0700); missing
// parent directories are created with 0777 less the umask. There is no fsync, like
// atomic.py: the guarantee is against an interrupted writer, not against power loss. The
// rename is os.Rename, which on Windows is MoveFileEx(MOVEFILE_REPLACE_EXISTING), the call
// behind Python's os.replace, so replacing an existing file works on both systems.
//
// The package imports nothing from this module, so any command can use it.
package atomicfile

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// rename is os.Rename and removeOld os.RemoveAll, replaceable by the tests
// to inject a failed swap or a failed removal of the old tree.
var (
	rename    = os.Rename
	removeOld = os.RemoveAll
)

// tempPattern is a CreateTemp/MkdirTemp pattern: prefix, random part,
// suffix. A "*" in the suffix would move the random part, so it is
// dropped (only the temp name changes, never the final one).
func tempPattern(prefix, suffix string) string {
	return prefix + "*" + strings.ReplaceAll(suffix, "*", "")
}

// lastRunes is s[-n:] in code points, as Python slices a str.
func lastRunes(s string, n int) string {
	r := []rune(s)
	if len(r) > n {
		r = r[len(r)-n:]
	}
	return string(r)
}

// WriteBytes writes data to path atomically (atomic.write_bytes): the
// parent directories are created, the file ends with mode 0644.
func WriteBytes(path string, data []byte) (err error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	dir := filepath.Dir(abs)
	if err := os.MkdirAll(dir, 0o777); err != nil {
		return err
	}
	f, err := os.CreateTemp(dir, tempPattern(".tmp-", lastRunes(filepath.Base(abs), 40)))
	if err != nil {
		return err
	}
	tmp := f.Name()
	defer func() {
		if r := recover(); r != nil {
			_ = os.Remove(tmp)
			panic(r)
		}
		if err != nil {
			_ = os.Remove(tmp)
		}
	}()
	if _, err = f.Write(data); err != nil {
		_ = f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	if err = os.Chmod(tmp, 0o644); err != nil { // CreateTemp makes 0600
		return err
	}
	return rename(tmp, path)
}

// WriteText writes text to path atomically, UTF-8, no newline translation.
func WriteText(path, text string) error { return WriteBytes(path, []byte(text)) }

// RestoreError reports a failed swap whose restore failed too: neither error
// hides the other, and the previous tree is still at Kept. It unwraps to the
// swap error, as Python chains the OSError from it.
type RestoreError struct {
	Final, Kept   string
	Swap, Restore error
}

func (e *RestoreError) Error() string {
	return fmt.Sprintf("swapping in the rebuilt %s failed (%v); restoring the previous tree failed too (%v): it is kept at %s",
		e.Final, e.Swap, e.Restore, e.Kept)
}

func (e *RestoreError) Unwrap() error { return e.Swap }

// RebuiltDir builds the tree final in an empty sibling temp directory
// (atomic.rebuilt_dir): build receives that directory; when it returns nil
// the new tree replaces final (the old tree is renamed aside, then
// removed). When build fails or panics, or the swap fails, the temp tree is
// removed and final is left exactly as it was (the aside is renamed back);
// when that restore fails too the error is a *RestoreError. Removing the
// old tree after a good swap ignores errors, as atomic.py's
// shutil.rmtree(old, ignore_errors=True) does: the new tree is in place, so
// the rebuild succeeded; a leftover <tmp>.old-<pid> tree is not reported.
//
// One intended difference: atomic.py leaves the temp tree behind when the
// chmod or the rename of the old tree aside fails; here it is removed too.
func RebuiltDir(final string, build func(tmp string) error) (err error) {
	final, err = filepath.Abs(final)
	if err != nil {
		return err
	}
	parent := filepath.Dir(final)
	if err := os.MkdirAll(parent, 0o777); err != nil {
		return err
	}
	tmp, err := os.MkdirTemp(parent, tempPattern("."+filepath.Base(final)+".tmp-", ""))
	if err != nil {
		return err
	}
	swapped := false
	defer func() {
		if swapped {
			return
		}
		_ = os.RemoveAll(tmp)
		if r := recover(); r != nil {
			panic(r)
		}
	}()
	if err := build(tmp); err != nil {
		return err
	}
	if err := os.Chmod(tmp, 0o755); err != nil { // MkdirTemp makes 0700
		return err
	}
	old := ""
	if st, err := os.Stat(final); err == nil && st.IsDir() {
		old = fmt.Sprintf("%s.old-%d", tmp, os.Getpid())
		if err := rename(final, old); err != nil {
			return err
		}
	}
	if swapErr := rename(tmp, final); swapErr != nil {
		if old != "" {
			if restoreErr := rename(old, final); restoreErr != nil {
				return &RestoreError{Final: final, Kept: old, Swap: swapErr, Restore: restoreErr}
			}
		}
		return swapErr
	}
	swapped = true
	if old != "" {
		_ = removeOld(old) // ignore_errors=True, like atomic.py
	}
	return nil
}
