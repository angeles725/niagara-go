package bog

import (
	"archive/zip"
	"bytes"
	"compress/bzip2"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"strings"
	"syscall"
	"unicode"
	"unicode/utf8"

	"github.com/angeles725/niagara-go/internal/pycompat"
)

// RootTag is the root element every BOG document carries in its first 64 KiB.
const RootTag = "<bajaObjectGraph"

// MaxNesting bounds config.bog-inside-ZIP nesting: .dist -> config.bog -> file.xml needs 1; the
// margin covers a re-zipped backup.
const MaxNesting = 3

// MaxMemberBytes caps the declared uncompressed size of the member read (real bogs are tens of MB).
const MaxMemberBytes = 1 << 30

// ExitBadInput is the exit code bog-nav (and other bogio callers) use for a *ReadError (D17).
const ExitBadInput = 3

const rootWindow = 65536

// zip general-purpose flag bits and the newest extract version Python's zipfile accepts.
const (
	flagEncrypted        = 0x1
	flagCompressedPatch  = 0x20
	flagStrongEncryption = 0x40
	maxExtractVersion    = 63
	methodBzip2          = 12 // Python's zipfile reads it; Go's archive/zip needs it registered
)

var memberLimit int64 = MaxMemberBytes // a variable so tests can lower it

// ReadError reports an input that is not a readable BOG document. Its message is the one
// tools/lib/bogio.py BogReadError carries; Err is the underlying library error, when any.
type ReadError struct {
	Msg string
	Err error
}

func (e *ReadError) Error() string { return e.Msg }

func (e *ReadError) Unwrap() error { return e.Err }

func readErr(err error, format string, args ...any) *ReadError {
	return &ReadError{Msg: fmt.Sprintf(format, args...), Err: err}
}

// ReadText returns the BOG XML of path as text: UTF-8, undecodable bytes replaced by U+FFFD
// exactly as Python's errors="replace" does.
func ReadText(path string) (string, error) {
	data, err := ReadBytes(path)
	if err != nil {
		return "", err
	}
	return pycompat.DecodeReplace(data), nil
}

// ReadBytes returns the BOG XML bytes of path. The input kind is detected by content: a
// config.bog ZIP holding file.xml, a .dist backup ZIP holding a nested config.bog, or a plain
// BOG XML file. Every failure is a *ReadError.
func ReadBytes(path string) ([]byte, error) {
	if st, err := os.Stat(path); err != nil || !st.Mode().IsRegular() {
		return nil, readErr(nil, "no such file: %s", path)
	}
	data, err := readFile(path)
	if err != nil {
		return nil, err
	}
	if len(bytes.Trim(data, " \t\n\r\v\f")) == 0 {
		return nil, readErr(nil, "empty BOG document in %s", path)
	}
	if !bytes.Contains(data[:min(len(data), rootWindow)], []byte(RootTag)) {
		return nil, readErr(nil, "%s is not a BOG document (no %s root element)", path, RootTag)
	}
	return data, nil
}

func readFile(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, osErr(path, err)
	}
	defer f.Close()
	head := make([]byte, 4)
	n, err := io.ReadFull(f, head)
	if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) && !errors.Is(err, io.EOF) {
		return nil, osErr(path, err)
	}
	if !bytes.HasPrefix(head[:n], []byte("PK")) {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, osErr(path, err)
		}
		return data, nil
	}
	st, err := f.Stat()
	if err != nil {
		return nil, osErr(path, err)
	}
	zr, err := openZip(f, st.Size(), path)
	if err != nil {
		return nil, err
	}
	return fromZip(zr, path, 0)
}

// osErr renders an OS error the way Python prints an OSError: "[Errno 13] Permission denied: 'p'".
func osErr(path string, err error) *ReadError {
	detail := err.Error()
	var pe *fs.PathError
	var errno syscall.Errno
	if errors.As(err, &pe) && errors.As(pe.Err, &errno) {
		msg := errno.Error()
		if r, size := utf8.DecodeRuneInString(msg); size > 0 {
			msg = string(unicode.ToUpper(r)) + msg[size:]
		}
		detail = fmt.Sprintf("[Errno %d] %s: %s", int(errno), msg, pycompat.StrRepr(pe.Path))
	}
	return readErr(err, "cannot read %s: %s", path, detail)
}

func openZip(r io.ReaderAt, size int64, label string) (*zip.Reader, error) {
	zr, err := zip.NewReader(r, size)
	if errors.Is(err, zip.ErrInsecurePath) {
		err = nil // the names are only compared, never used as paths
	}
	if err != nil {
		return nil, readErr(err, "corrupt or unsupported ZIP %s: File is not a zip file", label)
	}
	zr.RegisterDecompressor(methodBzip2, func(r io.Reader) io.ReadCloser {
		return io.NopCloser(bzip2.NewReader(r))
	})
	for _, f := range zr.File {
		if f.ReaderVersion > maxExtractVersion {
			return nil, readErr(nil, "corrupt or unsupported ZIP %s: zip file version %.1f",
				label, float64(f.ReaderVersion)/10)
		}
	}
	return zr, nil
}

// pickMember mirrors bogio._pick_member: file.xml, else the first name ending in file.xml; only
// without either, the first name ending in config.bog.
func pickMember(zr *zip.Reader) (member, inner string) {
	for _, f := range zr.File {
		if f.Name == "file.xml" {
			return f.Name, ""
		}
	}
	for _, f := range zr.File {
		if strings.HasSuffix(f.Name, "file.xml") {
			return f.Name, ""
		}
	}
	for _, f := range zr.File {
		if strings.HasSuffix(f.Name, "config.bog") {
			return "", f.Name
		}
	}
	return "", ""
}

func fromZip(zr *zip.Reader, label string, depth int) ([]byte, error) {
	member, inner := pickMember(zr)
	if member != "" {
		return readMember(zr, member, label)
	}
	if inner == "" {
		return nil, readErr(nil, "no file.xml or config.bog inside %s", label)
	}
	if depth >= MaxNesting {
		return nil, readErr(nil, "%s nests config.bog more than %d levels deep", label, MaxNesting)
	}
	data, err := readMember(zr, inner, label)
	if err != nil {
		return nil, err
	}
	nested, err := openZip(bytes.NewReader(data), int64(len(data)), "nested "+inner+" of "+label)
	if err != nil {
		return nil, err
	}
	return fromZip(nested, inner+" of "+label, depth+1)
}

// readMember reads the LAST entry called name (Python's getinfo), refusing an oversized one
// before reading and mapping archive failures to bogio's messages.
func readMember(zr *zip.Reader, name, label string) ([]byte, error) {
	var f *zip.File
	for _, cand := range zr.File {
		if cand.Name == name {
			f = cand
		}
	}
	if f.UncompressedSize64 > uint64(memberLimit) {
		return nil, readErr(nil, "%s of %s is too large (%d bytes > %d)", name, label, f.UncompressedSize64, memberLimit)
	}
	where := name + " of " + label
	fail := func(err error, detail string) ([]byte, error) {
		return nil, readErr(err, "corrupt or unsupported ZIP %s: %s", where, detail)
	}
	switch {
	case f.Flags&flagCompressedPatch != 0:
		return fail(nil, "compressed patched data (flag bit 5)")
	case f.Flags&flagStrongEncryption != 0:
		return fail(nil, "strong encryption (flag bit 6)")
	case f.Flags&flagEncrypted != 0:
		return fail(nil, fmt.Sprintf("File %s is encrypted, password required for extraction", pycompat.StrRepr(name)))
	}
	rc, err := f.Open()
	if errors.Is(err, zip.ErrAlgorithm) {
		return fail(err, "That compression method is not supported")
	}
	if err != nil {
		return fail(err, "Bad magic number for file header")
	}
	defer rc.Close()
	data, err := io.ReadAll(rc)
	if errors.Is(err, zip.ErrChecksum) {
		return fail(err, fmt.Sprintf("Bad CRC-32 for file %s", pycompat.StrRepr(name)))
	}
	if err != nil {
		return fail(err, err.Error())
	}
	return data, nil
}
