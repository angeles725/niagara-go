package blockfiles

import (
	"encoding/binary"
	"fmt"
	"os"
)

// stamp is what the walk cache compares of a directory: mtime and mode everywhere,
// ctime, inode and device where the system reports them (zero elsewhere).
type stamp struct {
	mtime, ctime int64
	ino, dev     uint64
	mode         uint32
}

const stampSize = 8 + 8 + 8 + 8 + 4

// statStamp is the stamp of path (os.Stat: a symlinked root is its target).
func statStamp(path string) (stamp, error) {
	st, err := os.Stat(path)
	if err != nil {
		return stamp{}, err
	}
	s := stamp{mtime: st.ModTime().UnixNano(), mode: uint32(st.Mode())}
	s.ctime, s.ino, s.dev = sysStamp(st)
	return s, nil
}

// newest is the later of mtime and ctime, in Unix nanoseconds.
func (s stamp) newest() int64 { return max(s.mtime, s.ctime) }

func (s stamp) append(b []byte) []byte {
	le := binary.LittleEndian
	b = le.AppendUint64(b, uint64(s.mtime))
	b = le.AppendUint64(b, uint64(s.ctime))
	b = le.AppendUint64(b, s.ino)
	b = le.AppendUint64(b, s.dev)
	return le.AppendUint32(b, s.mode)
}

// parseStamp decodes stampSize bytes (the zero stamp for a short slice, which the
// caller's reader has already flagged).
func parseStamp(b []byte) stamp {
	if len(b) != stampSize {
		return stamp{}
	}
	le := binary.LittleEndian
	return stamp{int64(le.Uint64(b)), int64(le.Uint64(b[8:])), le.Uint64(b[16:]), le.Uint64(b[24:]),
		le.Uint32(b[32:])}
}

// fileStamp is the stamp of the executable as a string, "" when it cannot be stat'ed.
func fileStamp(path string, err error) string {
	if err != nil {
		return ""
	}
	st, err := os.Stat(path)
	if err != nil {
		return ""
	}
	ctime, ino, dev := sysStamp(st)
	return fmt.Sprintf("%s:%d:%d:%d:%d:%d", path, st.Size(), st.ModTime().UnixNano(), ctime, ino, dev)
}
