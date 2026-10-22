package pycompat

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// Makedirs is os.makedirs(name, mode, exist_ok): make each missing parent, then mkdir name,
// failing with the error of the os.Mkdir that failed so OSErrorText renders Python's text.
// Unlike os.MkdirAll it reports an existing file as EEXIST ("[Errno 17] File exists") on the
// name given, and a file in the way of a parent as ENOTDIR on the first directory that could
// not be made, not on the file. A parent that already exists (FileExistsError) is not an
// error, and exist_ok only forgives a name that is a directory afterwards.
func Makedirs(name string, mode fs.FileMode, existOK bool) error {
	head, tail := pySplit(name)
	if tail == "" {
		head, tail = pySplit(head)
	}
	if head != "" && tail != "" && !pathExists(head) {
		if err := Makedirs(head, mode, existOK); err != nil && !errors.Is(err, fs.ErrExist) {
			return err
		}
		if tail == "." {
			return nil
		}
	}
	if err := os.Mkdir(name, mode); err != nil {
		if !existOK || !pathIsDir(name) {
			return err
		}
	}
	return nil
}

// pySplit is os.path.split: the tail after the last separator and the head before it, with
// the head's trailing separators removed unless it is only separators (after the volume).
func pySplit(p string) (head, tail string) {
	dir, tail := filepath.Split(p)
	vol := filepath.VolumeName(dir)
	seps := "/" // ntpath also takes '/' as a separator
	if filepath.Separator != '/' {
		seps += string(filepath.Separator)
	}
	rest := strings.TrimRight(dir[len(vol):], seps)
	if rest == "" {
		rest = dir[len(vol):]
	}
	return vol + rest, tail
}

func pathExists(p string) bool { _, err := os.Stat(p); return err == nil }

func pathIsDir(p string) bool { fi, err := os.Stat(p); return err == nil && fi.IsDir() }
