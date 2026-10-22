package pycompat

import (
	"archive/zip"
	"io"
)

// ReadZipMember is ZipFile.read(name) over the entries of an open archive: the bytes of the
// last entry match accepts, as zipfile's name index keeps the last entry of a repeated name.
// ok is false when no entry matches or the entry cannot be read. match says how the caller
// names an entry (the raw archive/zip name, or its zipfile decoding).
func ReadZipMember(files []*zip.File, match func(*zip.File) bool) ([]byte, bool) {
	var hit *zip.File
	for _, f := range files {
		if match(f) {
			hit = f
		}
	}
	if hit == nil {
		return nil, false
	}
	return ReadZipFile(hit)
}

// ReadZipFile is ZipFile.read(info): every byte of the entry, ok false on any open or read
// failure (a bad compression method, a corrupt stream, a CRC mismatch).
func ReadZipFile(f *zip.File) ([]byte, bool) {
	rc, err := f.Open()
	if err != nil {
		return nil, false
	}
	defer rc.Close()
	data, err := io.ReadAll(rc)
	if err != nil {
		return nil, false
	}
	return data, true
}
