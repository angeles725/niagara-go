package testkit

import (
	"archive/zip"
	"bytes"
	"encoding/binary"
	"hash/crc32"
	"os"
	"path/filepath"
	"testing"
)

// The zip end-record sizes (sizeEndCentDir, sizeEndCentDir64 of the reader),
// spelled here so the fixtures need not import the reader they test.
const (
	endRecordSize   = 22
	endRecord64Size = 56
)

// WriteJar writes a zip with archive/zip, one entry per (name, data) pair in
// order: a name ending in "/" is a directory entry, a repeated name is a
// duplicate entry (archive/zip does not refuse them, neither does Python).
func WriteJar(t *testing.T, path string, files ...[2]string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	z := zip.NewWriter(&buf)
	for _, f := range files {
		w, err := z.Create(f[0])
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(f[1])); err != nil {
			t.Fatal(err)
		}
	}
	if err := z.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
}

// RawEntry is one stored entry of a hand-built zip (RawZip): the raw name
// bytes, the general-purpose flags, the extract version, the central
// directory extra field and, when Size64 is set, a file_size field of
// 0xFFFFFFFF whose real value lives in that extra field.
//
// The entry-reader corners: LocalName replaces the name in the local header
// only, Method is the compression method written to both headers (the data
// stays raw), CRCXor flips CRC bits in both headers, ShortSize (> 0) makes
// the central file_size that many bytes (with their CRC), CompPlus is
// added to the central compress_size, and SharedHeader (> 0) points the
// entry at the local header of entry SharedHeader-1 instead of a new one.
type RawEntry struct {
	Name         string
	Data         string
	Flags        uint16
	Extract      uint8
	Extra        []byte
	Size64       bool
	LocalName    string
	Method       uint16
	CRCXor       uint32
	ShortSize    int
	CompPlus     uint32
	SharedHeader int
}

// RawZip builds a zip by hand, so the tests reach the central-directory
// corners archive/zip cannot write (raw cp437 names, a UTF-8 flag on bytes
// that are not UTF-8, extract versions, zip64 and Unicode-path extra
// fields, prepended data, an archive comment).
func RawZip(prefix string, entries []RawEntry, comment string) []byte {
	le16 := func(b *bytes.Buffer, v uint16) { _ = binary.Write(b, binary.LittleEndian, v) }
	le32 := func(b *bytes.Buffer, v uint32) { _ = binary.Write(b, binary.LittleEndian, v) }
	var body, cd bytes.Buffer
	offsets := make([]uint32, len(entries))
	for i, e := range entries {
		extract := e.Extract
		if extract == 0 {
			extract = 20
		}
		crc := crc32.ChecksumIEEE([]byte(e.Data)) ^ e.CRCXor
		size := uint32(len(e.Data))
		if e.ShortSize > 0 {
			size = uint32(e.ShortSize)
			crc = crc32.ChecksumIEEE([]byte(e.Data[:e.ShortSize])) ^ e.CRCXor
		}
		if e.Size64 {
			size = 0xFFFFFFFF
		}
		local := e.Name
		if e.LocalName != "" {
			local = e.LocalName
		}
		offset := uint32(body.Len())
		if e.SharedHeader > 0 {
			offset = offsets[e.SharedHeader-1]
		} else {
			body.WriteString("PK\x03\x04")
			le16(&body, uint16(extract))
			le16(&body, e.Flags)
			le16(&body, e.Method)
			le32(&body, 0) // time, date
			le32(&body, crc)
			le32(&body, uint32(len(e.Data)))
			le32(&body, uint32(len(e.Data)))
			le16(&body, uint16(len(local)))
			le16(&body, 0)
			body.WriteString(local)
			body.WriteString(e.Data)
		}
		offsets[i] = offset

		cd.WriteString("PK\x01\x02")
		cd.WriteByte(20) // create version
		cd.WriteByte(3)  // create system
		cd.WriteByte(extract)
		cd.WriteByte(0)
		le16(&cd, e.Flags)
		le16(&cd, e.Method)
		le32(&cd, 0)
		le32(&cd, crc)
		le32(&cd, uint32(len(e.Data))+e.CompPlus)
		le32(&cd, size)
		le16(&cd, uint16(len(e.Name)))
		le16(&cd, uint16(len(e.Extra)))
		le16(&cd, 0)
		le16(&cd, 0)
		le16(&cd, 0)
		le32(&cd, 0)
		le32(&cd, offset)
		cd.WriteString(e.Name)
		cd.Write(e.Extra)
	}
	var out bytes.Buffer
	out.WriteString(prefix)
	out.Write(body.Bytes())
	out.Write(cd.Bytes())
	out.WriteString("PK\x05\x06")
	le16(&out, 0)
	le16(&out, 0)
	le16(&out, uint16(len(entries)))
	le16(&out, uint16(len(entries)))
	le32(&out, uint32(cd.Len()))
	le32(&out, uint32(body.Len()))
	le16(&out, uint16(len(comment)))
	out.WriteString(comment)
	return out.Bytes()
}

// ExtraField is one (type, data) record of a zip extra field.
func ExtraField(tp uint16, data []byte) []byte {
	var b bytes.Buffer
	_ = binary.Write(&b, binary.LittleEndian, tp)
	_ = binary.Write(&b, binary.LittleEndian, uint16(len(data)))
	b.Write(data)
	return b.Bytes()
}

// UnicodePath is an Info-ZIP Unicode Path extra field (0x7075) naming
// `name` for an entry whose raw name is `raw`.
func UnicodePath(raw, name string) []byte {
	var d bytes.Buffer
	d.WriteByte(1)
	_ = binary.Write(&d, binary.LittleEndian, crc32.ChecksumIEEE([]byte(raw)))
	d.WriteString(name)
	return ExtraField(0x7075, d.Bytes())
}

// Zip64Size is a zip64 extra field (0x0001) carrying the file size.
func Zip64Size(n uint64) []byte {
	var d bytes.Buffer
	_ = binary.Write(&d, binary.LittleEndian, n)
	return ExtraField(0x0001, d.Bytes())
}

// Zip64Zip is RawZip(prefix, entries, "") with a zip64 end record and its
// locator before the end record, whose own size and offset fields are
// 0xFFFFFFFF. disks is the locator's disk count, reloffShift moves the
// locator's record offset away from the record (0 = where it is, counted
// without the prefix, as a zip written before the prefix was added says).
func Zip64Zip(prefix string, entries []RawEntry, disks uint32, reloffShift int64) []byte {
	plain := RawZip("", entries, "")
	le := binary.LittleEndian
	eocd := plain[len(plain)-endRecordSize:]
	cdSize, cdOff, n := uint64(le.Uint32(eocd[12:])), uint64(le.Uint32(eocd[16:])), uint64(le.Uint16(eocd[10:]))
	var out bytes.Buffer
	w := func(v any) { _ = binary.Write(&out, binary.LittleEndian, v) }
	out.WriteString(prefix)
	out.Write(plain[:len(plain)-endRecordSize])
	out.WriteString("PK\x06\x06")
	w(uint64(endRecord64Size - 12))
	w(uint16(45))
	w(uint16(45))
	w(uint32(0))
	w(uint32(0))
	w(n)
	w(n)
	w(cdSize)
	w(cdOff)
	out.WriteString("PK\x06\x07")
	w(uint32(0))
	w(uint64(int64(cdOff+cdSize) + reloffShift))
	w(disks)
	out.WriteString("PK\x05\x06")
	w(uint16(0xFFFF))
	w(uint16(0xFFFF))
	w(uint16(0xFFFF))
	w(uint16(0xFFFF))
	w(uint32(0xFFFFFFFF))
	w(uint32(0xFFFFFFFF))
	w(uint16(0))
	return out.Bytes()
}

// WithEntryCount sets both entry counts of a zip's end record to n: the
// directory is walked by its byte size, never by these counts.
func WithEntryCount(zip []byte, n uint16) []byte {
	out := append([]byte(nil), zip...)
	eocd := out[len(out)-endRecordSize:]
	binary.LittleEndian.PutUint16(eocd[8:], n)
	binary.LittleEndian.PutUint16(eocd[10:], n)
	return out
}
