package qnx6read

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/angeles725/niagara-go/internal/pycompat"
)

const (
	blockSize   = 1024
	inodeSize   = 128
	noBlock     = 0xffffffff
	sbMagic     = 0x68191122
	sbOffset    = 0x2000
	sbSize      = 0x1000
	maxDepth    = 40
	dirMode     = 0x4000
	typeMask    = 0xf000
	maxProbeOff = 40
)

// Partitions are the byte offsets of the QNX6 partitions on the JACE-8000 SD card.
var Partitions = map[string]int64{"P2": 135266304, "P3": 3711959040}

// rootNode is a superblock root: file size, 16 block pointers and the indirection levels.
type rootNode struct {
	size   uint64
	ptr    []uint32
	levels int
}

type inode struct {
	size   uint64
	mode   uint16
	ptr    []uint32
	levels int
}

// qnx6 mirrors the Python QNX6 class: the logical block b lives at part + (b+off)*1024.
type qnx6 struct {
	r          io.ReaderAt
	image      string // the image name, for I/O error messages
	part       int64
	off        int64
	blocksize  uint32
	numInodes  uint32
	inodeRN    rootNode
	longRN     rootNode
	inodeTable []byte
	truncated  []string
	visited    map[uint32]string // directory inode -> the path it was listed under
}

// ioError is a failed read of the image (not a short read at its end).
type ioError struct {
	image string
	pos   int64
	err   error
}

func (e *ioError) Error() string {
	return fmt.Sprintf("cannot read %s at byte %d: %s", e.image, e.pos, pycompat.StrerrorOf(e.err))
}

func (e *ioError) Unwrap() error { return e.err }

// readAt reads up to n bytes at pos; like Python file.read it returns fewer at the end of the file.
func (q *qnx6) readAt(pos int64, n int) ([]byte, error) {
	buf := make([]byte, n)
	k, err := q.r.ReadAt(buf, pos)
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, &ioError{q.image, pos, err}
	}
	return buf[:k], nil
}

func (q *qnx6) rawBlock(b uint32, off int64) ([]byte, error) {
	return q.readAt(q.part+(int64(b)+off)*blockSize, blockSize)
}

func u32s(b []byte, n int) []uint32 {
	out := make([]uint32, n)
	for i := range out {
		out[i] = binary.LittleEndian.Uint32(b[4*i:])
	}
	return out
}

func parseRoot(sb []byte, off int) rootNode {
	return rootNode{size: binary.LittleEndian.Uint64(sb[off:]), ptr: u32s(sb[off+8:], 16), levels: int(sb[off+72])}
}

// open reads the superblock, detects the block offset and loads the inode and long-name files.
func open(r io.ReaderAt, part int64, image string) (*qnx6, error) {
	q := &qnx6{r: r, image: image, part: part}
	sb, err := q.readAt(part+sbOffset, sbSize)
	if err != nil {
		return nil, err
	}
	notQNX6 := func(why string) error {
		return fmt.Errorf("not a QNX6 partition at byte %d of %s (%s)", part, image, why)
	}
	if len(sb) < 4 {
		return nil, notQNX6(fmt.Sprintf("superblock truncated: %d of %d bytes", len(sb), sbSize))
	}
	if m := binary.LittleEndian.Uint32(sb); m != sbMagic {
		return nil, notQNX6(fmt.Sprintf("superblock magic 0x%08x", m))
	}
	if len(sb) < 305 {
		return nil, notQNX6(fmt.Sprintf("superblock truncated: %d of %d bytes", len(sb), sbSize))
	}
	q.blocksize = binary.LittleEndian.Uint32(sb[48:])
	q.numInodes = binary.LittleEndian.Uint32(sb[52:])
	q.inodeRN, q.longRN = parseRoot(sb, 72), parseRoot(sb, 232)
	if q.off, err = q.detectOff(); err != nil {
		return nil, err
	}
	if q.off < 0 {
		return nil, fmt.Errorf("could not detect block offset in %s", image)
	}
	if q.inodeTable, err = q.readFile(q.inodeRN.size, q.inodeRN.ptr, q.inodeRN.levels); err != nil {
		return nil, err
	}
	if _, err = q.readFile(q.longRN.size, q.longRN.ptr, q.longRN.levels); err != nil {
		return nil, err
	}
	return q, nil
}

// detectOff tries offsets 0..39 until logical block 0 of the inode file holds a directory
// inode of a plausible size; it returns -1 when none does. A format error at one offset tries the
// next (Python: any exception); a failed read of the image is returned.
func (q *qnx6) detectOff() (int64, error) {
	rn := q.inodeRN
	for off := int64(0); off < maxProbeOff; off++ {
		b0, err := q.rawBlock(rn.ptr[0], off)
		if err != nil {
			return 0, err
		}
		if len(b0) < 128 {
			continue
		}
		q.off = off
		data0, err := q.logicalBlock(rn.ptr, rn.levels, 0)
		var ioErr *ioError
		if errors.As(err, &ioErr) {
			return 0, err
		}
		if err != nil || len(data0) < 34 {
			continue
		}
		mode := binary.LittleEndian.Uint16(data0[32:])
		size := binary.LittleEndian.Uint64(data0)
		if mode&typeMask == dirMode && size > 0 && size < 50_000_000 {
			return off, nil
		}
	}
	return -1, nil
}

var errStop = errors.New("stop")

// leafBlocks visits the ordered data-block numbers of ptrs with `levels` of indirection, lazily
// like the Python generator; visit returns errStop to end the walk.
func (q *qnx6) leafBlocks(ptrs []uint32, levels int, visit func(uint32) error) error {
	for _, p := range ptrs {
		if p == noBlock {
			continue
		}
		if levels == 0 {
			if err := visit(p); err != nil {
				return err
			}
			continue
		}
		blk, err := q.rawBlock(p, q.off)
		if err != nil {
			return err
		}
		if len(blk) != blockSize {
			return fmt.Errorf("indirect block %d is truncated (%d bytes)", p, len(blk))
		}
		if err := q.leafBlocks(u32s(blk, blockSize/4), levels-1, visit); err != nil {
			return err
		}
	}
	return nil
}

func (q *qnx6) logicalBlock(ptrs []uint32, levels int, n int) ([]byte, error) {
	var got []byte
	i := 0
	err := q.leafBlocks(ptrs, levels, func(b uint32) error {
		if i == n {
			var err error
			if got, err = q.rawBlock(b, q.off); err != nil {
				return err
			}
			return errStop
		}
		i++
		return nil
	})
	if errors.Is(err, errStop) {
		return got, nil
	}
	if err != nil {
		return nil, err
	}
	return nil, fmt.Errorf("logical block %d is past the end of the file", n)
}

// readFile concatenates the data blocks of a file up to size. A block cut short by the end of the
// image is an error unless the file needs no more bytes than it holds (Python returned the short
// file silently).
func (q *qnx6) readFile(size uint64, ptrs []uint32, levels int) ([]byte, error) {
	var out []byte
	err := q.leafBlocks(ptrs, levels, func(b uint32) error {
		blk, err := q.rawBlock(b, q.off)
		if err != nil {
			return err
		}
		if len(blk) < blockSize && uint64(len(out)+len(blk)) < size {
			return fmt.Errorf("data block %d is truncated: %d of %d bytes (the image ends)", b, len(blk), blockSize)
		}
		out = append(out, blk...)
		if uint64(len(out)) >= size {
			return errStop
		}
		return nil
	})
	if err != nil && !errors.Is(err, errStop) {
		return nil, err
	}
	if uint64(len(out)) > size {
		out = out[:size]
	}
	return out, nil
}

func (q *qnx6) inode(n uint32) (inode, error) {
	start := (int64(n) - 1) * inodeSize
	if n == 0 || start+101 > int64(len(q.inodeTable)) {
		return inode{}, fmt.Errorf("inode %d is outside the %d-byte inode table", n, len(q.inodeTable))
	}
	e := q.inodeTable[start:min(start+inodeSize, int64(len(q.inodeTable)))]
	return inode{size: binary.LittleEndian.Uint64(e), mode: binary.LittleEndian.Uint16(e[32:]),
		ptr: u32s(e[36:], 16), levels: int(e[100])}, nil
}

func (q *qnx6) inodeData(n uint32) ([]byte, error) {
	i, err := q.inode(n)
	if err != nil {
		return nil, err
	}
	return q.readFile(i.size, i.ptr, i.levels)
}

// latin1 decodes bytes as ISO-8859-1 (Python decode('latin1')).
func latin1(b []byte) string {
	var sb strings.Builder
	for _, c := range b {
		sb.WriteRune(rune(c))
	}
	return sb.String()
}

func (q *qnx6) longName(idx uint32) (string, error) {
	rec, err := q.logicalBlock(q.longRN.ptr, q.longRN.levels, int(idx))
	if err != nil {
		return "", err
	}
	if len(rec) < 2 {
		return "", fmt.Errorf("long name %d is truncated", idx)
	}
	n := int(binary.LittleEndian.Uint16(rec))
	return latin1(rec[2:min(2+n, len(rec))]), nil
}

type dirent struct {
	name string
	ino  uint32
}

func (q *qnx6) listdir(ino uint32) ([]dirent, error) {
	data, err := q.inodeData(ino)
	if err != nil {
		return nil, err
	}
	var out []dirent
	for o := 0; o+32 <= len(data); o += 32 {
		ent := data[o : o+32]
		de := binary.LittleEndian.Uint32(ent)
		if de == 0 {
			continue
		}
		var name string
		if n := int(ent[4]); n == 0xff {
			if name, err = q.longName(binary.LittleEndian.Uint32(ent[8:])); err != nil {
				return nil, err
			}
		} else {
			name = latin1(ent[5:min(5+n, 32)])
		}
		if name != "." && name != ".." {
			out = append(out, dirent{name, de})
		}
	}
	return out, nil
}

// resolve finds the inode of an absolute path; ok is false when a component is missing.
func (q *qnx6) resolve(path string) (ino uint32, ok bool, err error) {
	ino = 1
	for _, part := range strings.Split(path, "/") {
		if part == "" {
			continue
		}
		ents, err := q.listdir(ino)
		if err != nil {
			return 0, false, err
		}
		found := false
		for _, e := range ents {
			if e.name == part {
				ino, found = e.ino, true
				break
			}
		}
		if !found {
			return 0, false, nil
		}
	}
	return ino, true, nil
}

// walk visits (path, inode, isdir) depth first in sorted name order. A directory deeper than
// maxDepth is not listed; its path goes to q.truncated. A directory inode reached a second time
// (a loop, or two entries naming one directory) is an error: Python walked a loop down to
// maxDepth, and two sibling entries for one inode doubled the walk at every level.
func (q *qnx6) walk(ino uint32, path string, depth int, visit func(string, inode, bool)) error {
	if depth == 0 {
		q.truncated = nil
		q.visited = map[uint32]string{ino: "/"}
	}
	if depth > maxDepth {
		q.truncated = append(q.truncated, path)
		return nil
	}
	ents, err := q.listdir(ino)
	if err != nil {
		return err
	}
	sort.Slice(ents, func(a, b int) bool {
		if ents[a].name != ents[b].name {
			return ents[a].name < ents[b].name
		}
		return ents[a].ino < ents[b].ino
	})
	for _, e := range ents {
		i, err := q.inode(e.ino)
		if err != nil {
			return err
		}
		p := path + "/" + e.name
		isDir := i.mode&typeMask == dirMode
		if isDir {
			if prev, seen := q.visited[e.ino]; seen {
				return fmt.Errorf("directory cycle: %s is inode %d, already listed as %s", p, e.ino, prev)
			}
			q.visited[e.ino] = p
		}
		visit(p, i, isDir)
		if isDir {
			if err := q.walk(e.ino, p, depth+1, visit); err != nil {
				return err
			}
		}
	}
	return nil
}
