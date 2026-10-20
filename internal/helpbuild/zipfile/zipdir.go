package zipfile

import (
	"bytes"
	"encoding/binary"
	"errors"
	"hash/crc32"
	"io"
	"math"
	"math/big"
	"math/bits"
	"os"
	"runtime"
	"sort"
	"strings"
	"unicode/utf8"
)

// The central-directory reader of Python's zipfile (3.15 ZipFile(path,
// "r").infolist(): _EndRecData, _EndRecData64, _RealGetContents,
// ZipInfo._decodeExtra), ported rather than taken from archive/zip so a jar
// lists the same entries, names and sizes, and is unreadable in the same
// cases: archive/zip keeps raw name bytes (Python decodes cp437 unless the
// UTF-8 flag is set, cuts at a NUL and honours the 0x7075 Unicode-path extra
// field) and counts records where Python walks the directory's byte size.

const (
	sizeEndCentDir          = 22
	sizeCentralDir          = 46
	sizeEndCentDir64Locator = 20
	sizeEndCentDir64        = 56
	zipMaxComment           = 1<<16 - 1
	maxExtractVersion       = 63
	maskUTFFilename         = 1 << 11
)

var (
	ErrBadZip = errors.New("bad zip file") // BadZipFile, OSError, ValueError
	// ErrUnsupportedZip is an entry needing a zip version above 6.3: Python
	// raises NotImplementedError (uncaught by read_listing, so the Python
	// tool stops); here the jar is unreadable.
	ErrUnsupportedZip = errors.New("zip file version above 6.3")
)

// cp437High decodes bytes 0x80-0xFF ("cp437" codec; 0x00-0x7F is ASCII).
const cp437High = "ÇüéâäàåçêëèïîìÄÅÉæÆôöòûùÿÖÜ¢£¥₧ƒáíóúñÑªº¿⌐¬½¼¡«»░▒▓│┤╡╢╖╕╣║╗╝╜╛┐" +
	"└┴┬├─┼╞╟╚╔╩╦╠═╬╧╨╤╥╙╘╒╓╫╪┘┌█▄▌▐▀αßΓπΣσµτΦΘΩδ∞φε∩≡±≥≤⌠⌡÷≈°∙·√ⁿ²■ "

var cp437Table = []rune(cp437High)

func decodeCP437(b []byte) string {
	var sb strings.Builder
	for _, c := range b {
		if c < 0x80 {
			sb.WriteByte(c)
		} else {
			sb.WriteRune(cp437Table[c-0x80])
		}
	}
	return sb.String()
}

// sanitizeName is zipfile._sanitize_filename: the name ends at the first
// NUL, and on Windows (os.sep "\\") backslashes become "/".
func sanitizeName(name string) string {
	if i := strings.IndexByte(name, 0); i >= 0 {
		name = name[:i]
	}
	if runtime.GOOS == "windows" {
		name = strings.ReplaceAll(name, `\`, "/")
	}
	return name
}

// Info is the part of a ZipInfo the listing and the entry reader use:
// name is the normalized filename, origName the decoded central-directory
// name before _sanitize_filename (ZipFile.open compares the local header
// against it), headerOffset includes the prepended-data shift ("concat")
// and endOffset is _end_offset (the next local header, or the directory).
type Info struct {
	Name, origName string
	Size           uint64
	CRC            uint32
	flags, method  uint16
	compressSize   uint64
	headerOffset   int64
	endOffset      int64
	rawOffset      uint64 // header offset before the shift, zip64-extended
}

// readAt is fp.seek(off); fp.read(n): up to n bytes, fewer at the end.
func readAt(f *os.File, size, off int64, n uint64) ([]byte, error) {
	if off >= size {
		return nil, nil
	}
	if left := uint64(size - off); n > left {
		n = left
	}
	buf := make([]byte, n)
	m, err := f.ReadAt(buf, off)
	if err != nil && err != io.EOF {
		return nil, ErrBadZip
	}
	return buf[:m], nil
}

// endRec holds what _RealGetContents uses of the end record: the size of
// the central directory and where the end record (or zip64 record) starts.
type endRec struct {
	size     uint64
	offset   uint64
	location int64
}

// endRecData is _EndRecData: nil when no end record is found.
func endRecData(f *os.File, size int64) (*endRec, error) {
	if size < sizeEndCentDir {
		return nil, nil // seek(-22, 2) raises OSError: return None
	}
	data, err := readAt(f, size, size-sizeEndCentDir, sizeEndCentDir)
	if err != nil {
		return nil, err
	}
	if len(data) == sizeEndCentDir && bytes.HasPrefix(data, []byte("PK\x05\x06")) && data[20] == 0 && data[21] == 0 {
		return endRecData64(f, size, size-sizeEndCentDir, unpackEnd(data, size-sizeEndCentDir))
	}
	start := size - zipMaxComment - sizeEndCentDir
	if start < 0 {
		start = 0
	}
	data, err = readAt(f, size, start, zipMaxComment+sizeEndCentDir)
	if err != nil {
		return nil, err
	}
	i := bytes.LastIndex(data, []byte("PK\x05\x06"))
	if i < 0 || len(data)-i < sizeEndCentDir {
		return nil, nil
	}
	return endRecData64(f, size, start+int64(i), unpackEnd(data[i:i+sizeEndCentDir], start+int64(i)))
}

func unpackEnd(d []byte, location int64) *endRec {
	le := binary.LittleEndian
	return &endRec{size: uint64(le.Uint32(d[12:])), offset: uint64(le.Uint32(d[16:])), location: location}
}

// endRecData64 is _EndRecData64: the zip64 end record, when its locator
// precedes the end record, replaces the directory size, offset and location.
func endRecData64(f *os.File, size, offset int64, rec *endRec) (*endRec, error) {
	le := binary.LittleEndian
	offset -= sizeEndCentDir64Locator
	if offset < 0 {
		return rec, nil
	}
	data, err := readAt(f, size, offset, sizeEndCentDir64Locator)
	if err != nil || len(data) != sizeEndCentDir64Locator {
		return nil, ErrBadZip
	}
	if !bytes.HasPrefix(data, []byte("PK\x06\x07")) {
		return rec, nil
	}
	diskno, reloff, disks := le.Uint32(data[4:]), le.Uint64(data[8:]), le.Uint32(data[16:])
	if diskno != 0 || disks > 1 {
		return nil, ErrBadZip // multi-disk zip files are not supported
	}
	offset -= sizeEndCentDir64
	if offset < 0 || reloff > uint64(offset) {
		return nil, ErrBadZip
	}
	extrasz := offset - int64(reloff)
	data, err = readAt(f, size, int64(reloff), sizeEndCentDir64)
	if err != nil || len(data) != sizeEndCentDir64 {
		return nil, ErrBadZip
	}
	if !bytes.HasPrefix(data, []byte("PK\x06\x06")) && int64(reloff) != offset {
		extrasz = 0 // prepended data: assume no zip64 extensible data
		if data, err = readAt(f, size, offset, sizeEndCentDir64); err != nil || len(data) != sizeEndCentDir64 {
			return nil, ErrBadZip
		}
	}
	if !bytes.HasPrefix(data, []byte("PK\x06\x06")) {
		return nil, ErrBadZip
	}
	sz, dirsize, diroffset := le.Uint64(data[4:]), le.Uint64(data[40:]), le.Uint64(data[48:])
	end, carry := bits.Add64(diroffset, dirsize, 0)
	sz12, carry12 := bits.Add64(sz, 12, 0)
	if carry != 0 || end != reloff || carry12 != 0 || sz12 != uint64(sizeEndCentDir64+extrasz) {
		return nil, ErrBadZip
	}
	return &endRec{size: dirsize, offset: diroffset, location: offset - extrasz}, nil
}

// File is an open ZipFile(path, "r"): the file, its size and infolist().
type File struct {
	f     *os.File
	size  int64
	Infos []Info
}

// Close closes the file.
func (z *File) Close() error { return z.f.Close() }

// ReadCentralDir is ZipFile(path).infolist(). Any error means Python's
// read_listing returns None.
func ReadCentralDir(path string) ([]Info, error) {
	z, err := Open(path)
	if err != nil {
		return nil, err
	}
	defer z.f.Close()
	return z.Infos, nil
}

// Open is ZipFile(path, "r"): _RealGetContents, then the _end_offset of
// every entry. The caller closes z.f.
func Open(path string) (*File, error) {
	st, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if st.IsDir() {
		return nil, ErrBadZip // open() raises IsADirectoryError
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	z, err := readZip(f, st.Size())
	if err != nil {
		f.Close()
		return nil, err
	}
	return z, nil
}

func readZip(f *os.File, size int64) (*File, error) {
	rec, err := endRecData(f, size)
	if err != nil {
		return nil, err
	}
	if rec == nil || rec.size > uint64(rec.location) {
		return nil, ErrBadZip // not a zip file / bad offset for central directory
	}
	startDir := rec.location - int64(rec.size)
	data, err := readAt(f, size, startDir, rec.size)
	if err != nil {
		return nil, err
	}
	infos, err := parseCentralDir(data, rec.size)
	if err != nil {
		return nil, err
	}
	setOffsets(infos, rec, startDir)
	return &File{f: f, size: size, Infos: infos}, nil
}

// setOffsets adds _handle_prepended_data's concat (location - size_cd -
// offset_cd) to every header offset, then sets _end_offset: entries sorted
// by header offset (stable), walked in reverse from the directory start.
// Offsets outside int64 (only absurd zip64 values) are clamped: -1 and
// MaxInt64 both make ZipFile.open fail, as Python's seek/read would.
func setOffsets(infos []Info, rec *endRec, startDir int64) {
	concat := new(big.Int).Sub(big.NewInt(startDir), new(big.Int).SetUint64(rec.offset))
	for i := range infos {
		off := new(big.Int).Add(new(big.Int).SetUint64(infos[i].rawOffset), concat)
		switch {
		case off.Sign() < 0:
			infos[i].headerOffset = -1
		case !off.IsInt64():
			infos[i].headerOffset = math.MaxInt64
		default:
			infos[i].headerOffset = off.Int64()
		}
	}
	order := make([]int, len(infos))
	for i := range order {
		order[i] = i
	}
	sort.SliceStable(order, func(a, b int) bool { return infos[order[a]].headerOffset < infos[order[b]].headerOffset })
	end := startDir
	for k := len(order) - 1; k >= 0; k-- {
		x := &infos[order[k]]
		x.endOffset = end
		end = x.headerOffset
	}
}

// parseCentralDir is the record loop of _RealGetContents over the bytes of
// the directory (fewer than sizeCD when the file is truncated).
func parseCentralDir(data []byte, sizeCD uint64) ([]Info, error) {
	le := binary.LittleEndian
	pos := 0
	read := func(n int) []byte { // BytesIO.read(n)
		end := pos + n
		if end > len(data) {
			end = len(data)
		}
		b := data[pos:end]
		pos = end
		return b
	}
	var out []Info
	for total := uint64(0); total < sizeCD; {
		cd := read(sizeCentralDir)
		if len(cd) != sizeCentralDir || !bytes.HasPrefix(cd, []byte("PK\x01\x02")) {
			return nil, ErrBadZip // truncated / bad magic number for central directory
		}
		nameLen, extraLen, commentLen := le.Uint16(cd[28:]), le.Uint16(cd[30:]), le.Uint16(cd[32:])
		raw := read(int(nameLen))
		var name string
		if le.Uint16(cd[8:])&maskUTFFilename != 0 {
			if !utf8.Valid(raw) {
				return nil, ErrBadZip // UnicodeDecodeError, a ValueError
			}
			name = string(raw)
		} else {
			name = decodeCP437(raw)
		}
		x := Info{Name: sanitizeName(name), origName: name, CRC: le.Uint32(cd[16:]), Size: uint64(le.Uint32(cd[24:])),
			flags: le.Uint16(cd[8:]), method: le.Uint16(cd[10:]),
			compressSize: uint64(le.Uint32(cd[20:])), rawOffset: uint64(le.Uint32(cd[42:]))}
		extra := read(int(extraLen))
		read(int(commentLen))
		if cd[6] > maxExtractVersion {
			return nil, ErrUnsupportedZip
		}
		if err := decodeExtra(&x, extra, crc32.ChecksumIEEE(raw)); err != nil {
			return nil, err
		}
		out = append(out, x)
		total += sizeCentralDir + uint64(nameLen) + uint64(extraLen) + uint64(commentLen)
	}
	return out, nil
}

// decodeExtra is ZipInfo._decodeExtra: zip64 sizes (0x0001) and the
// Info-ZIP Unicode path (0x7075, used when its CRC matches the raw name).
func decodeExtra(x *Info, extra []byte, nameCRC uint32) error {
	le := binary.LittleEndian
	for len(extra) >= 4 {
		tp, ln := le.Uint16(extra), int(le.Uint16(extra[2:]))
		if ln+4 > len(extra) {
			return ErrBadZip // corrupt extra field
		}
		data := extra[4 : ln+4]
		switch tp {
		case 0x0001:
			for _, field := range []*uint64{&x.Size, &x.compressSize, &x.rawOffset} {
				full := field == &x.Size && *field == 0xFFFF_FFFF_FFFF_FFFF
				if !full && *field != 0xFFFF_FFFF {
					continue
				}
				if len(data) < 8 {
					return ErrBadZip // corrupt zip64 extra field
				}
				*field = le.Uint64(data)
				data = data[8:]
			}
		case 0x7075:
			if len(data) < 5 {
				return ErrBadZip
			}
			if data[0] == 1 && le.Uint32(data[1:]) == nameCRC {
				if !utf8.Valid(data[5:]) {
					return ErrBadZip
				}
				if len(data) > 5 { // an empty one only warns in Python
					x.Name = sanitizeName(string(data[5:]))
				}
			}
		}
		extra = extra[ln+4:]
	}
	return nil
}
