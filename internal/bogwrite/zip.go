package bogwrite

import (
	"archive/zip"
	"bytes"
	"compress/flate"
	"hash/crc32"
	"os"
	"runtime"
	"time"
)

// writeZip is zipfile.ZipFile(path, "w", ZIP_DEFLATED).writestr("file.xml", xml): one
// deflated member stamped with the local time now, sizes in the local header (no data
// descriptor, no extra field), version made by 2.0 on Unix (or MS-DOS on Windows) and
// external attributes 0o600 << 16.
func writeZip(path string, xml []byte) error {
	var deflated bytes.Buffer
	fw, err := flate.NewWriter(&deflated, flate.DefaultCompression)
	if err != nil {
		return err
	}
	if _, err = fw.Write(xml); err == nil {
		err = fw.Close()
	}
	if err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0o666)
	if err != nil {
		return osError(err, path)
	}
	system := uint16(3) // Unix
	if runtime.GOOS == "windows" {
		system = 0
	}
	date, clock := dosTime(time.Now())
	// The legacy MS-DOS fields are deliberate: Modified would add an extended-timestamp
	// extra field that Python's zipfile never writes.
	fh := &zip.FileHeader{
		Name: "file.xml", Method: zip.Deflate, CreatorVersion: system<<8 | 20, ReaderVersion: 20,
		//lint:ignore SA1019 Python zipfile parity (no extended-timestamp extra field)
		ModifiedDate: date, ModifiedTime: clock, ExternalAttrs: 0o600 << 16,
		CRC32: crc32.ChecksumIEEE(xml), CompressedSize64: uint64(deflated.Len()), UncompressedSize64: uint64(len(xml)),
	}
	z := zip.NewWriter(f)
	w, err := z.CreateRaw(fh)
	if err == nil {
		_, err = w.Write(deflated.Bytes())
	}
	if err == nil {
		err = z.Close()
	}
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	return err
}

// dosTime is the MS-DOS date and time (2-second resolution) of t in local time.
func dosTime(t time.Time) (uint16, uint16) {
	date := uint16(t.Day() + int(t.Month())<<5 + (t.Year()-1980)<<9)
	clock := uint16(t.Second()/2 + t.Minute()<<5 + t.Hour()<<11)
	return date, clock
}
