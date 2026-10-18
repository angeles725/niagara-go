package qnx6read

import (
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"testing"

	"github.com/angeles725/niagara-go/internal/cli"
	"github.com/angeles725/niagara-go/internal/golden"
)

// The image checks are argparse errors in Python ("qnx6read.py: error: <message>"); the message
// part is ours to match before internal/argparse lands.
func TestValidateImageMatchesGoldenUsageMessages(t *testing.T) {
	cases := map[string]string{"usage-no-image": "", "usage-missing-image": "images/missing.img",
		"usage-directory-image": "images", "usage-char-device": "/dev/null"}
	stderr := map[string][]byte{}
	for _, suite := range []string{root, filepath.Join(root, "linux")} {
		frozen, err := golden.Load(suite)
		if err != nil {
			t.Fatal(err)
		}
		for _, c := range frozen {
			stderr[c.Name] = c.Stderr
		}
	}
	t.Chdir(runDir)
	for name, image := range cases {
		if name == "usage-char-device" {
			// This guard is what protects Windows: there /dev/null does not exist (Stat fails),
			// so the Linux-only character-device case is skipped instead of failing.
			if st, err := os.Stat(image); err != nil || st.Mode()&os.ModeCharDevice == 0 {
				continue
			}
		}
		msg := ValidateImage(image, false)
		if want := "qnx6read.py: error: " + msg + "\n"; msg == "" || !bytes.HasSuffix(stderr[name], []byte(want)) {
			t.Errorf("%s: ValidateImage = %q; golden stderr %q", name, msg, stderr[name])
		}
	}
	if msg := ValidateImage("images/p2.img", false); msg != "" {
		t.Errorf("a regular image is refused: %s", msg)
	}
	if st, err := os.Stat("/dev/null"); err == nil && st.Mode()&os.ModeCharDevice != 0 {
		if msg := ValidateImage("/dev/null", true); msg != "" {
			t.Errorf("--char-device still refuses /dev/null: %s", msg)
		}
	}
}

func TestValidateImageUnreadable(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("mode 0 only sets the read-only attribute on Windows: the file stays readable")
	}
	if os.Geteuid() == 0 {
		t.Skip("root reads anything")
	}
	img := filepath.Join(t.TempDir(), "disk.img")
	if err := os.WriteFile(img, []byte("x"), 0); err != nil {
		t.Fatal(err)
	}
	if msg := ValidateImage(img, false); !strings.Contains(msg, "cannot read image") ||
		!strings.Contains(msg, "Permission denied") {
		t.Errorf("got %q", msg)
	}
}

func TestLatin1(t *testing.T) {
	if got := latin1([]byte("caf\xe9")); got != "café" {
		t.Errorf("latin1 = %q", got)
	}
}

// memInode is one inode of a synthetic image: its size, mode, data block numbers and levels.
type memInode struct {
	size   uint64
	mode   uint16
	blocks []uint32
	levels int
}

// memFS builds a qnx6 over an in-memory image (partition at byte 0, block offset 0) whose
// block b holds blocks[b]; imageLen trims the image (0 keeps it whole).
func memFS(inodes []memInode, blocks map[uint32][]byte, imageLen int) *qnx6 {
	top := uint32(0)
	for b := range blocks {
		top = max(top, b+1)
	}
	img := make([]byte, int(top)*blockSize)
	for b, data := range blocks {
		copy(img[int(b)*blockSize:], data)
	}
	if imageLen > 0 {
		img = img[:imageLen]
	}
	table := make([]byte, len(inodes)*inodeSize)
	for n, in := range inodes {
		e := table[n*inodeSize:]
		binary.LittleEndian.PutUint64(e, in.size)
		binary.LittleEndian.PutUint16(e[32:], in.mode)
		for k := 0; k < 16; k++ {
			p := uint32(noBlock)
			if k < len(in.blocks) {
				p = in.blocks[k]
			}
			binary.LittleEndian.PutUint32(e[36+4*k:], p)
		}
		e[100] = byte(in.levels)
	}
	return &qnx6{r: bytes.NewReader(img), inodeTable: table}
}

// dirBlock is a directory data block holding the given (name, inode) entries.
func dirBlock(ents ...dirent) []byte {
	b := make([]byte, blockSize)
	for k, e := range ents {
		d := b[32*k:]
		binary.LittleEndian.PutUint32(d, e.ino)
		d[4] = byte(len(e.name))
		copy(d[5:], e.name)
	}
	return b
}

const fileMode = 0x81a4

func runList(q *qnx6) (int, string, string) {
	var out, errb bytes.Buffer
	code := list(q, cli.Stdio{Out: &out, Err: &errb})
	return code, out.String(), errb.String()
}

// A walk that hits a corrupt inode reports it with exit 2 after the partial listing.
func TestCorruptInodeIsExit2(t *testing.T) {
	q := memFS([]memInode{{size: 64, mode: dirMode | 0o755, blocks: []uint32{1}}},
		map[uint32][]byte{1: dirBlock(dirent{"bad", 999})}, 0)
	code, out, errb := runList(q)
	if code != cli.ExitCannotSearch || out != "" ||
		errb != "qnx6read: inode 999 is outside the 128-byte inode table\n" {
		t.Errorf("code %d stdout %q stderr %q", code, out, errb)
	}
}

// A directory reached twice (a loop, or two entries naming one directory) stops the walk with
// exit 2 instead of listing it again; two sibling entries for one inode would otherwise double
// the walk at every level.
func TestDirectoryCycleIsAnError(t *testing.T) {
	dir := uint16(dirMode | 0o755)
	loop := memFS([]memInode{{size: 64, mode: dir, blocks: []uint32{1}}, {size: 64, mode: dir, blocks: []uint32{2}}},
		map[uint32][]byte{1: dirBlock(dirent{"a", 2}), 2: dirBlock(dirent{"up", 1})}, 0)
	code, out, errb := runList(loop)
	if code != cli.ExitCannotSearch || out != "d         64 /a\n" ||
		errb != "qnx6read: directory cycle: /a/up is inode 1, already listed as /\n" {
		t.Errorf("loop: code %d stdout %q stderr %q", code, out, errb)
	}
	twice := memFS([]memInode{{size: 64, mode: dir, blocks: []uint32{1}}, {size: 0, mode: dir}},
		map[uint32][]byte{1: dirBlock(dirent{"a", 2}, dirent{"b", 2})}, 0)
	code, out, errb = runList(twice)
	if code != cli.ExitCannotSearch || out != "d          0 /a\n" ||
		errb != "qnx6read: directory cycle: /b is inode 2, already listed as /a\n" {
		t.Errorf("twice: code %d stdout %q stderr %q", code, out, errb)
	}
}

// File data that runs past the end of the image is an error, not a silently short file.
func TestTruncatedFileIsAnError(t *testing.T) {
	q := memFS([]memInode{{size: 2048, mode: fileMode, blocks: []uint32{0, 1}}},
		map[uint32][]byte{0: bytes.Repeat([]byte("a"), blockSize), 1: bytes.Repeat([]byte("b"), blockSize)},
		blockSize+100)
	data, err := q.inodeData(1)
	if err == nil || err.Error() != "data block 1 is truncated: 100 of 1024 bytes (the image ends)" {
		t.Fatalf("got %d bytes, err %v", len(data), err)
	}
	// The same short block is fine when the file needs no more than it holds.
	q = memFS([]memInode{{size: 1100, mode: fileMode, blocks: []uint32{0, 1}}},
		map[uint32][]byte{0: bytes.Repeat([]byte("a"), blockSize), 1: bytes.Repeat([]byte("b"), blockSize)},
		blockSize+100)
	if data, err := q.inodeData(1); err != nil || len(data) != 1100 {
		t.Fatalf("got %d bytes, err %v", len(data), err)
	}
}

// failingReader serves the superblock and the first probe block, then fails every read.
type failingReader struct {
	sb    []byte
	calls int
}

func (f *failingReader) ReadAt(p []byte, off int64) (int, error) {
	f.calls++
	switch {
	case off == sbOffset:
		return copy(p, f.sb), nil
	case f.calls == 2:
		return len(p), nil
	}
	return 0, syscall.EIO
}

// An I/O error while probing block offsets is reported, not read as "no offset fits".
func TestDetectOffPropagatesIOError(t *testing.T) {
	sb := make([]byte, sbSize)
	binary.LittleEndian.PutUint32(sb, sbMagic)
	binary.LittleEndian.PutUint32(sb[80:], 7) // inode file root: block 7, levels 0
	want := "cannot read disk.img at byte 7168: Input/output error"
	if runtime.GOOS == "windows" {
		// syscall.EIO is a value Go invents on Windows, not a Win32 code: CPython's
		// winerror_to_errno default (EINVAL) words it.
		want = "cannot read disk.img at byte 7168: Invalid argument"
	}
	_, err := open(&failingReader{sb: sb}, 0, "disk.img")
	if err == nil || err.Error() != want {
		t.Fatalf("err = %v, want %q", err, want)
	}
}

func TestUnknownPartitionIsExit2(t *testing.T) {
	t.Chdir(runDir)
	var out, errb bytes.Buffer
	code := Run(Options{Image: "images/p2.img", Part: "P9"}, cli.Stdio{Out: &out, Err: &errb})
	if code != cli.ExitCannotSearch || errb.String() != "qnx6read: unknown partition P9 (known: P2, P3)\n" {
		t.Errorf("code %d stderr %q", code, errb.String())
	}
}

// The indirect and double-indirect golden fixtures really go through indirection: big.dat
// (inode 6) has one level and huge.dat (inode 7) two, and both decode to their own content.
func TestFixtureIndirection(t *testing.T) {
	f, err := os.Open(filepath.Join(runDir, "images", "p2.img"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	q, err := open(f, Partitions["P2"], "p2.img")
	if err != nil {
		t.Fatal(err)
	}
	for ino, levels := range map[uint32]int{6: 1, 7: 2} {
		in, err := q.inode(ino)
		if err != nil || in.levels != levels {
			t.Fatalf("inode %d: levels %d (%v), want %d", ino, in.levels, err, levels)
		}
		top, err := q.rawBlock(in.ptr[0], q.off)
		if err != nil {
			t.Fatal(err)
		}
		data, err := q.inodeData(ino)
		if err != nil || uint64(len(data)) != in.size || bytes.HasPrefix(top, data[:64]) {
			t.Fatalf("inode %d: %d bytes (%v), or the data is the pointer block itself", ino, len(data), err)
		}
	}
}

// A deep tree without a cycle still stops at maxDepth with the INCOMPLETE exit 1.
func TestDepthLimitStillApplies(t *testing.T) {
	var inodes []memInode
	blocks := map[uint32][]byte{}
	for n := uint32(1); n <= maxDepth+3; n++ {
		inodes = append(inodes, memInode{size: 32, mode: dirMode | 0o755, blocks: []uint32{n}})
		blocks[n] = dirBlock(dirent{"d", n + 1})
	}
	inodes = append(inodes, memInode{mode: dirMode | 0o755})
	code, out, errb := runList(memFS(inodes, blocks, 0))
	if code != cli.ExitNoMatch || strings.Count(out, "\n") != maxDepth+1 ||
		!strings.Contains(errb, "INCOMPLETE: depth limit hit, 1 directorie(s) not listed") {
		t.Errorf("code %d, %d lines, stderr %q", code, strings.Count(out, "\n"), errb)
	}
}
