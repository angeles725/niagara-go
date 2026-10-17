// Package qnx6read is a minimal read-only QNX6 (Power-Safe) reader for the JACE-8000 SD image
// (port of niagara-research tools/qnx6read.py, block B674): it lists the directory tree of one
// partition of a raw disk image, or extracts one file.
package qnx6read

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"sort"
	"strings"
	"syscall"

	"github.com/angeles725/niagara-go/internal/cli"
)

// Extract names one file to copy out of the image.
type Extract struct {
	Src string // absolute path inside the partition
	Dst string // output file
}

// Options is the parsed command line; Image has passed ValidateImage and Part is a key of
// Partitions.
type Options struct {
	Image   string   // raw disk image or block device
	Part    string   // "P2" or "P3"
	Extract *Extract // nil lists the tree
}

func isImage(path string, charOK bool) bool {
	st, err := os.Stat(path)
	if err != nil {
		return false
	}
	m := st.Mode()
	isChar := m&fs.ModeCharDevice != 0
	isBlock := m&fs.ModeDevice != 0 && !isChar
	return m.IsRegular() || isBlock || charOK && isChar
}

// ValidateImage returns the usage error for an image argument ("" when it is usable): a regular
// file or a block device; a character device only with charDevice (--char-device).
func ValidateImage(image string, charDevice bool) string {
	if image == "" {
		return "no image: pass --image <disk.img> or set QNX6_IMG"
	}
	if !isImage(image, charDevice) {
		if isImage(image, true) {
			return fmt.Sprintf("image is a character device (a terminal? /dev/null?): %s; pass --char-device "+
				"for a raw disk that is one", image)
		}
		return fmt.Sprintf("image not found (need a file or a block device): %s", image)
	}
	f, err := os.Open(image)
	if err != nil {
		return fmt.Sprintf("cannot read image %s: %s", image, strerror(err))
	}
	f.Close()
	return ""
}

// strerror renders an I/O error the way Python's OSError.strerror does.
func strerror(err error) string {
	var errno syscall.Errno
	if errors.As(err, &errno) {
		if msg := errno.Error(); msg != "" {
			return strings.ToUpper(msg[:1]) + msg[1:]
		}
	}
	return err.Error()
}

// fail writes "qnx6read: <message>" to stderr and returns code.
func fail(s cli.Stdio, code int, format string, a ...any) int {
	fmt.Fprintf(s.Err, "qnx6read: "+format+"\n", a...)
	return code
}

// Run executes one qnx6read invocation and returns its exit code: 0 ok, 1 listing incomplete
// (depth limit) or extract source not found, 2 an image that is not a readable QNX6 partition
// (or an unknown partition name).
func Run(o Options, s cli.Stdio) int {
	part, ok := Partitions[o.Part]
	if !ok {
		names := make([]string, 0, len(Partitions))
		for k := range Partitions {
			names = append(names, k)
		}
		sort.Strings(names)
		return fail(s, cli.ExitCannotSearch, "unknown partition %s (known: %s)", o.Part, strings.Join(names, ", "))
	}
	f, err := os.Open(o.Image)
	if err != nil {
		return fail(s, cli.ExitCannotSearch, "cannot read image %s: %s", o.Image, strerror(err))
	}
	defer f.Close()
	q, err := open(f, part, o.Image)
	if err != nil {
		return fail(s, cli.ExitCannotSearch, "%s", err)
	}
	fmt.Fprintf(s.Err, "%s OFF=%d blocksize=%d inodes=%d\n", o.Part, q.off, q.blocksize, q.numInodes)
	if o.Extract != nil {
		return extract(q, o.Extract, s)
	}
	return list(q, s)
}

// list prints the tree of q and the totals; a walk error ends it with exit 2.
func list(q *qnx6, s cli.Stdio) int {
	ndir, nfile := 0, 0
	err := q.walk(1, "", 0, func(p string, i inode, isDir bool) {
		t := "-"
		if isDir {
			t = "d"
			ndir++
		} else {
			nfile++
		}
		fmt.Fprintf(s.Out, "%s %10d %s\n", t, i.size, p)
	})
	if err != nil {
		return fail(s, cli.ExitCannotSearch, "%s", err)
	}
	fmt.Fprintf(s.Err, "totals: dirs=%d files=%d\n", ndir, nfile)
	if len(q.truncated) > 0 {
		fmt.Fprintf(s.Err, "INCOMPLETE: depth limit hit, %d directorie(s) not listed, e.g. %s\n",
			len(q.truncated), q.truncated[0])
		return cli.ExitNoMatch
	}
	return cli.ExitFound
}

func extract(q *qnx6, x *Extract, s cli.Stdio) int {
	ino, ok, err := q.resolve(x.Src)
	if err != nil {
		return fail(s, cli.ExitCannotSearch, "%s", err)
	}
	if !ok {
		return fail(s, cli.ExitNoMatch, "%s: not found in the image", x.Src)
	}
	data, err := q.inodeData(ino)
	if err != nil {
		return fail(s, cli.ExitCannotSearch, "%s", err)
	}
	if err := os.WriteFile(x.Dst, data, 0o666); err != nil {
		return fail(s, cli.ExitCannotSearch, "cannot write %s: %s", x.Dst, strerror(err))
	}
	fmt.Fprintf(s.Err, "extracted %s -> %s (%d bytes)\n", x.Src, x.Dst, len(data))
	return cli.ExitFound
}
