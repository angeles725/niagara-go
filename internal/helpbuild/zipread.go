package helpbuild

import (
	"bytes"
	"compress/flate"
	"encoding/binary"
	"errors"
	"fmt"
	"hash/crc32"
	"io"
	"math"
	"unicode/utf8"
)

// Reading one entry the way Python's ZipFile.open(info).read() does (3.15
// ZipFile.open and ZipExtFile): the local header is checked against the
// central directory, the compressed bytes are read, inflated, cut at the
// central file_size and CRC-checked. Every case where Python raises is an
// error here; the builders stop on it as the Python tools do.

const (
	sizeFileHeader      = 30
	maskEncrypted       = 1 << 0
	maskCompressedPatch = 1 << 5
	maskStrongEncrypt   = 1 << 6
	zipStored           = 0
	zipDeflated         = 8
)

var (
	// ErrEncryptedEntry is an entry with the encryption flag: Python raises
	// RuntimeError (no password is ever given).
	ErrEncryptedEntry = errors.New("encrypted zip entry, password required for extraction")
	// ErrUnsupportedMethod is a compression method other than stored or
	// deflated. Python also inflates bzip2 (12), lzma (14) and zstd (93);
	// Niagara jars use none of them, so they are not ported.
	ErrUnsupportedMethod = errors.New("zip compression method not supported")
	errTruncatedEntry    = errors.New("zip entry data truncated") // EOFError
)

// dataStart is ZipFile.open(info) for reading: the offset of the entry's
// compressed bytes, after the checks Python makes in its order.
func (z *zipFile) dataStart(x *zipInfo) (int64, error) {
	le := binary.LittleEndian
	if x.headerOffset < 0 {
		return 0, errBadZip // seek to a negative offset
	}
	hdr, err := readAt(z.f, z.size, x.headerOffset, sizeFileHeader)
	if err != nil || len(hdr) != sizeFileHeader {
		return 0, fmt.Errorf("%w: truncated file header of %q", errBadZip, x.origName)
	}
	if !bytes.HasPrefix(hdr, []byte("PK\x03\x04")) {
		return 0, fmt.Errorf("%w: bad magic number for the file header of %q", errBadZip, x.origName)
	}
	fname, err := readAt(z.f, z.size, x.headerOffset+sizeFileHeader, uint64(le.Uint16(hdr[26:])))
	if err != nil {
		return 0, err
	}
	if x.flags&maskCompressedPatch != 0 || x.flags&maskStrongEncrypt != 0 {
		return 0, fmt.Errorf("%w: %q uses compressed patched data or strong encryption", ErrUnsupportedZip, x.origName)
	}
	var local string
	if le.Uint16(hdr[6:])&maskUTFFilename != 0 {
		if !utf8.Valid(fname) {
			return 0, fmt.Errorf("%w: local name of %q is not UTF-8", errBadZip, x.origName)
		}
		local = string(fname)
	} else {
		local = decodeCP437(fname)
	}
	if local != x.origName {
		return 0, fmt.Errorf("%w: file name in directory %q and header %q differ", errBadZip, x.origName, local)
	}
	pos := x.headerOffset + sizeFileHeader + int64(len(fname)) + int64(le.Uint16(hdr[28:]))
	if pos > x.endOffset || x.compressSize > uint64(x.endOffset-pos) {
		if x.endOffset != x.headerOffset { // equal: Python only warns
			return 0, fmt.Errorf("%w: overlapped entries: %q (possible zip bomb)", errBadZip, x.origName)
		}
	}
	if x.flags&maskEncrypted != 0 {
		return 0, fmt.Errorf("%w: %q", ErrEncryptedEntry, x.origName)
	}
	if x.method != zipStored && x.method != zipDeflated {
		return 0, fmt.Errorf("%w: %q uses method %d", ErrUnsupportedMethod, x.origName, x.method)
	}
	return pos, nil
}

// readEntry returns the bytes Python reads from the entry. whole is
// read() (fidelity, fill_missing: every compressed byte is inflated, then
// the output is cut at file_size); otherwise it is shutil.copyfileobj's
// bounded reads (extract), which stop once file_size bytes came out. The
// CRC-32 of the result must be the central directory's.
func (z *zipFile) readEntry(x *zipInfo, whole bool) ([]byte, error) {
	pos, err := z.dataStart(x)
	if err != nil {
		return nil, err
	}
	comp, err := readAt(z.f, z.size, pos, x.compressSize)
	if err != nil {
		return nil, err
	}
	got := uint64(len(comp))
	if x.compressSize > 0 && got == 0 {
		return nil, fmt.Errorf("%w: %q", errTruncatedEntry, x.origName)
	}
	var out []byte
	switch x.method {
	case zipStored:
		if got < x.compressSize && got < x.size {
			return nil, fmt.Errorf("%w: %q", errTruncatedEntry, x.origName)
		}
		out = comp[:min(got, x.size)]
	case zipDeflated:
		r := flate.NewReader(bytes.NewReader(comp))
		if whole {
			out, err = io.ReadAll(r)
		} else {
			out, err = io.ReadAll(io.LimitReader(r, int64(min(x.size, math.MaxInt64))))
		}
		// Input used up mid-stream: zlib's flush ends the read when every
		// compressed byte was there; a short file is EOFError, as for a
		// stored entry.
		if errors.Is(err, io.ErrUnexpectedEOF) {
			if got < x.compressSize {
				return nil, fmt.Errorf("%w: %q", errTruncatedEntry, x.origName)
			}
			err = nil
		}
		if err != nil {
			return nil, fmt.Errorf("%w: inflating %q: %v", errBadZip, x.origName, err)
		}
		if uint64(len(out)) > x.size {
			out = out[:x.size]
		}
	}
	if crc32.ChecksumIEEE(out) != x.crc {
		return nil, fmt.Errorf("%w: bad CRC-32 for file %q", errBadZip, x.name)
	}
	return out, nil
}
