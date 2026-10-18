package helpbuild

import (
	"errors"
	"io"
	"io/fs"
	"os"
	"strings"

	"github.com/angeles725/niagara-go/internal/index"
	"github.com/angeles725/niagara-go/internal/pycompat"
)

// textChunk is the number of bytes TextIOWrapper decodes at a time
// (_CHUNK_SIZE; CPython 3.15 reads 8,192 per decode even with its 128 KiB
// buffer). It decides when a decode error is raised and the position it
// names, which is relative to the decoded chunk.
const textChunk = 8192

// pyTextLines is `for line in open(path, encoding="utf-8")`: strict UTF-8
// decoded chunk by chunk with an incremental decoder, universal newlines,
// each line passed with its "\n" (the last one may have none). visit
// returning false stops reading, like a `break` before the next line is
// needed. A decode error is reported when the chunk that holds it is read,
// with the position inside that chunk (plus the bytes of a sequence the
// previous chunk left incomplete), exactly as str(UnicodeDecodeError)
// prints it; an open or read failure is the OSError text.
func pyTextLines(path string, visit func(line string) bool) error {
	f, err := os.Open(path)
	if err != nil {
		return pyOSError(path, err)
	}
	defer f.Close()
	var pending []byte // an incomplete UTF-8 sequence at the end of the last chunk
	var text strings.Builder
	pendingCR := false
	buf := make([]byte, textChunk)
	for {
		n, rerr := io.ReadFull(f, buf)
		if rerr != nil && rerr != io.EOF && rerr != io.ErrUnexpectedEOF {
			return pyOSError(path, rerr)
		}
		final := n == 0
		data := append(pending, buf[:n]...)
		pending = nil
		decoded, rest, derr := decodeIncremental(data, final)
		if derr != nil {
			return derr
		}
		pending = rest
		// IncrementalNewlineDecoder(translate=True): a trailing "\r" waits
		// for the next chunk (it may start a "\r\n").
		s := decoded
		if pendingCR {
			s = "\r" + s
			pendingCR = false
		}
		if !final && strings.HasSuffix(s, "\r") {
			s, pendingCR = s[:len(s)-1], true
		}
		text.WriteString(pycompat.UniversalNewlines(s))
		all := text.String()
		for {
			i := strings.IndexByte(all, '\n')
			if i < 0 {
				break
			}
			if !visit(all[:i+1]) {
				return nil
			}
			all = all[i+1:]
		}
		text.Reset()
		text.WriteString(all)
		if final {
			if all != "" {
				visit(all)
			}
			return nil
		}
	}
}

// decodeIncremental is the C UTF-8 decoder's decode(data, final): the
// decoded text and, unless final, the bytes of a sequence cut by the end of
// data (kept for the next call); an invalid sequence is an error whose
// position counts from the start of data.
func decodeIncremental(data []byte, final bool) (string, []byte, error) {
	err := index.CheckUTF8(data)
	if err == nil {
		return string(data), nil, nil
	}
	var ue *index.UTF8Error
	if !final && errors.As(err, &ue) && ue.Reason == "unexpected end of data" && ue.End == len(data) {
		return string(data[:ue.Start]), append([]byte(nil), data[ue.Start:]...), nil
	}
	return "", nil, err
}

// pyOSError is the str() of the OSError Python raises for err on path.
func pyOSError(path string, err error) error {
	var pe *fs.PathError
	if errors.As(err, &pe) {
		return errors.New(pycompat.OSErrorText(&fs.PathError{Op: pe.Op, Path: path, Err: pe.Err}))
	}
	return err
}
