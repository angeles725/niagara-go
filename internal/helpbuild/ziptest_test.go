package helpbuild

import (
	"archive/zip"
	"bytes"
	"encoding/binary"
	"hash/crc32"
	"os"
	"path/filepath"
	"testing"
)

// writeJar writes a zip with archive/zip, one entry per (name, data) pair in
// order: a name ending in "/" is a directory entry, a repeated name is a
// duplicate entry (archive/zip does not refuse them, neither does Python).
func writeJar(t *testing.T, path string, files ...[2]string) {
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

// rawEntry is one stored entry of a hand-built zip (rawZip): the raw name
// bytes, the general-purpose flags, the extract version, the central
// directory extra field and, when size64 is set, a file_size field of
// 0xFFFFFFFF whose real value lives in that extra field.
type rawEntry struct {
	name    string
	data    string
	flags   uint16
	extract uint8
	extra   []byte
	size64  bool
}

// rawZip builds a zip by hand, so the tests reach the central-directory
// corners archive/zip cannot write (raw cp437 names, a UTF-8 flag on bytes
// that are not UTF-8, extract versions, zip64 and Unicode-path extra
// fields, prepended data, an archive comment).
func rawZip(prefix string, entries []rawEntry, comment string) []byte {
	le16 := func(b *bytes.Buffer, v uint16) { _ = binary.Write(b, binary.LittleEndian, v) }
	le32 := func(b *bytes.Buffer, v uint32) { _ = binary.Write(b, binary.LittleEndian, v) }
	var body, cd bytes.Buffer
	for _, e := range entries {
		extract := e.extract
		if extract == 0 {
			extract = 20
		}
		crc := crc32.ChecksumIEEE([]byte(e.data))
		size := uint32(len(e.data))
		if e.size64 {
			size = 0xFFFFFFFF
		}
		offset := uint32(body.Len())
		body.WriteString("PK\x03\x04")
		le16(&body, uint16(extract))
		le16(&body, e.flags)
		le16(&body, 0) // stored
		le32(&body, 0) // time, date
		le32(&body, crc)
		le32(&body, uint32(len(e.data)))
		le32(&body, uint32(len(e.data)))
		le16(&body, uint16(len(e.name)))
		le16(&body, 0)
		body.WriteString(e.name)
		body.WriteString(e.data)

		cd.WriteString("PK\x01\x02")
		cd.WriteByte(20) // create version
		cd.WriteByte(3)  // create system
		cd.WriteByte(extract)
		cd.WriteByte(0)
		le16(&cd, e.flags)
		le16(&cd, 0)
		le32(&cd, 0)
		le32(&cd, crc)
		le32(&cd, uint32(len(e.data)))
		le32(&cd, size)
		le16(&cd, uint16(len(e.name)))
		le16(&cd, uint16(len(e.extra)))
		le16(&cd, 0)
		le16(&cd, 0)
		le16(&cd, 0)
		le32(&cd, 0)
		le32(&cd, offset)
		cd.WriteString(e.name)
		cd.Write(e.extra)
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

// extraField is one (type, data) record of a zip extra field.
func extraField(tp uint16, data []byte) []byte {
	var b bytes.Buffer
	_ = binary.Write(&b, binary.LittleEndian, tp)
	_ = binary.Write(&b, binary.LittleEndian, uint16(len(data)))
	b.Write(data)
	return b.Bytes()
}

// unicodePath is an Info-ZIP Unicode Path extra field (0x7075) naming
// `name` for an entry whose raw name is `raw`.
func unicodePath(raw, name string) []byte {
	var d bytes.Buffer
	d.WriteByte(1)
	_ = binary.Write(&d, binary.LittleEndian, crc32.ChecksumIEEE([]byte(raw)))
	d.WriteString(name)
	return extraField(0x7075, d.Bytes())
}

// zip64Size is a zip64 extra field (0x0001) carrying the file size.
func zip64Size(n uint64) []byte {
	var d bytes.Buffer
	_ = binary.Write(&d, binary.LittleEndian, n)
	return extraField(0x0001, d.Bytes())
}

// zip64Zip is rawZip(prefix, entries, "") with a zip64 end record and its
// locator before the end record, whose own size and offset fields are
// 0xFFFFFFFF. disks is the locator's disk count, reloffShift moves the
// locator's record offset away from the record (0 = where it is, counted
// without the prefix, as a zip written before the prefix was added says).
func zip64Zip(prefix string, entries []rawEntry, disks uint32, reloffShift int64) []byte {
	plain := rawZip("", entries, "")
	le := binary.LittleEndian
	eocd := plain[len(plain)-sizeEndCentDir:]
	cdSize, cdOff, n := uint64(le.Uint32(eocd[12:])), uint64(le.Uint32(eocd[16:])), uint64(le.Uint16(eocd[10:]))
	var out bytes.Buffer
	w := func(v any) { _ = binary.Write(&out, binary.LittleEndian, v) }
	out.WriteString(prefix)
	out.Write(plain[:len(plain)-sizeEndCentDir])
	out.WriteString("PK\x06\x06")
	w(uint64(sizeEndCentDir64 - 12))
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

// withEntryCount sets both entry counts of a zip's end record to n: the
// directory is walked by its byte size, never by these counts.
func withEntryCount(zip []byte, n uint16) []byte {
	out := append([]byte(nil), zip...)
	eocd := out[len(out)-sizeEndCentDir:]
	binary.LittleEndian.PutUint16(eocd[8:], n)
	binary.LittleEndian.PutUint16(eocd[10:], n)
	return out
}
