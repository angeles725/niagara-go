// Package atomicfile writes files and directory trees so that an interrupted writer never
// leaves a half-written result behind (the Go side of niagara_help_lib/atomic.py).
//
// A file write goes to a temp file in the same directory (".tmp-*<last 40 code points of
// the name>"), then a rename swaps it in; on error or panic the temp file is removed and the
// previous file is untouched. WriteBytes and WriteText take the content, WriteStream a
// function that writes it; a builder that needs a path (a SQLite database) reserves one with
// TempPath and swaps it in with Replace. A rebuilt tree is built in an empty sibling temp directory
// and swapped in the same way, the old tree renamed aside and restored when the swap fails.
//
// Mode rules: a written file ends with mode 0644 (WriteStream: the mode it is given) and a
// rebuilt tree's root with 0755 (the explicit chmod calls of atomic.py; CreateTemp/MkdirTemp
// would leave 0600/0700); a TempPath file has 0666 less the umask; missing parent
// directories are created with 0777 less the umask. There is no fsync, like
// atomic.py: the guarantee is against an interrupted writer, not against power loss. The
// rename is os.Rename, which on Windows is MoveFileEx(MOVEFILE_REPLACE_EXISTING), the call
// behind Python's os.replace, so replacing an existing file works on both systems.
//
// The package imports nothing from this module, so any command can use it.
package atomicfile

import (
	"fmt"
	"io"
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
func WriteBytes(path string, data []byte) error {
	return WriteStream(path, 0o644, func(w io.Writer) error {
		_, err := w.Write(data)
		return err
	})
}

// WriteStream writes path atomically like WriteBytes, streaming: write fills
// the temp file (unbuffered: a caller that writes in small pieces wraps w in
// a bufio.Writer and flushes it), which ends with mode perm and is renamed
// over path only when write and every file operation succeeded. When write
// fails or panics the temp file is closed and removed, path is untouched,
// and a panic is raised again.
func WriteStream(path string, perm os.FileMode, write func(io.Writer) error) (err error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	dir := filepath.Dir(abs)
	if err := os.MkdirAll(dir, 0o777); err != nil {
		return err
	}
	f, err := os.CreateTemp(dir, tempName(abs))
	if err != nil {
		return err
	}
	tmp := f.Name()
	open := true
	defer func() {
		r := recover()
		if r == nil && err == nil {
			return
		}
		if open {
			_ = f.Close() // Windows cannot remove an open file
		}
		_ = os.Remove(tmp)
		if r != nil {
			panic(r)
		}
	}()
	if err = write(f); err != nil {
		return err
	}
	open = false
	if err = f.Close(); err != nil {
		return err
	}
	if err = os.Chmod(tmp, perm); err != nil { // CreateTemp makes 0600
		return err
	}
	return rename(tmp, path)
}

// tempName is the CreateTemp pattern of a temp file beside path:
// ".tmp-*<last 40 code points of the name>".
func tempName(path string) string {
	return tempPattern(".tmp-", lastRunes(filepath.Base(path), 40))
}

// TempPath reserves an empty file beside final, for a builder that needs a
// path rather than a writer (a SQLite database), and creates the missing
// parent directories. The file has the mode a plain create gives, 0666 less
// the umask, as SQLite and Python create a database (CreateTemp's 0600 would
// make the renamed file private to its owner): CreateTemp only reserves a
// unique name. Replace then swaps the built file in.
func TempPath(final string) (string, error) {
	dir := filepath.Dir(final)
	if err := os.MkdirAll(dir, 0o777); err != nil {
		return "", err
	}
	f, err := os.CreateTemp(dir, tempName(final))
	if err != nil {
		return "", err
	}
	name := f.Name()
	f.Close()
	if err := os.Remove(name); err != nil {
		return "", err
	}
	f, err = os.OpenFile(name, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0o666)
	if err != nil {
		return "", err
	}
	if err := f.Close(); err != nil {
		os.Remove(name)
		return "", err
	}
	return name, nil
}

// Replace builds final through the temp path tmp: build fills tmp, which is
// renamed over final only when build succeeds. When build fails or panics,
// or the rename fails, discard(tmp) runs (os.Remove when discard is nil, so
// a builder with sidecar files passes its own) and final is untouched; a
// panic is raised again.
func Replace(final, tmp string, build func(tmp string) error, discard func(tmp string)) (err error) {
	if discard == nil {
		discard = func(p string) { _ = os.Remove(p) }
	}
	defer func() {
		if r := recover(); r != nil {
			discard(tmp)
			panic(r)
		}
		if err != nil {
			discard(tmp)
		}
	}()
	if err = build(tmp); err != nil {
		return err
	}
	return rename(tmp, final)
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
