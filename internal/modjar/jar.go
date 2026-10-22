package modjar

import (
	"archive/zip"
	"errors"
	"io"
)

// Jar is one open jar: its entry names in archive order and their bytes on demand. It is
// for a caller that reads several entries of the same jar (course check); ReadMember is
// the one-member case built on it.
type Jar struct {
	zr    *zip.ReadCloser
	names []string
}

// Option changes how Open and ReadMember see a jar.
type Option func(*options)

type options struct{ name func(*zip.File) string }

// WithNameFunc makes fn the name of each entry (Names, Last, ReadMember's match and
// MetaInf) instead of the raw f.Name: for a caller that decodes names as Python's zipfile
// does (nav's core.ZipName: CP437 without the UTF-8 flag, cut at the first NUL).
func WithNameFunc(fn func(*zip.File) string) Option {
	return func(o *options) { o.name = fn }
}

// Open opens the jar at path. The error is zip.OpenReader's, unwrapped, so os.IsNotExist
// and zip.ErrFormat still tell a missing jar from a broken one; an insecure entry name is
// not an error (the reader is still good).
func Open(path string, opts ...Option) (*Jar, error) {
	o := options{name: func(f *zip.File) string { return f.Name }}
	for _, opt := range opts {
		opt(&o)
	}
	opens.Add(1)
	zr, err := zip.OpenReader(path)
	if err != nil && !errors.Is(err, zip.ErrInsecurePath) {
		return nil, err
	}
	names := make([]string, len(zr.File))
	for i, f := range zr.File {
		names[i] = o.name(f)
	}
	return &Jar{zr: zr, names: names}, nil
}

// Names is every entry's name in archive order; index i is the entry Read(i) reads.
func (j *Jar) Names() []string { return j.names }

// Last is the index of the last entry named name (zipfile.ZipFile.read's pick when a name
// repeats), -1 when there is none.
func (j *Jar) Last(name string) int {
	for i := len(j.names) - 1; i >= 0; i-- {
		if j.names[i] == name {
			return i
		}
	}
	return -1
}

// Read returns the bytes of entry i, at most limit of them when limit > 0. The error is the
// entry's open or read error, unwrapped (a bad method, a corrupt stream, a CRC mismatch).
func (j *Jar) Read(i int, limit int64) ([]byte, error) {
	rc, err := j.zr.File[i].Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	var r io.Reader = rc
	if limit > 0 {
		r = io.LimitReader(rc, limit)
	}
	return io.ReadAll(r)
}

// Close releases the jar.
func (j *Jar) Close() error { return j.zr.Close() }
