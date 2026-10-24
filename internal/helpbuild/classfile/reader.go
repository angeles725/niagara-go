package classfile

import "encoding/binary"

// reader is a big-endian cursor with a sticky error: the first fault is kept
// and every later read returns zero, so a parser checks err once per step.
// base is the file offset of b[0], so errors from a sub-reader still report
// file offsets.
type reader struct {
	b         []byte
	off, base int
	err       error
}

func (r *reader) pos() int { return r.base + r.off }

func (r *reader) fail(err error) {
	if r.err == nil {
		r.err = err
	}
}

// take returns the next n bytes, or nil and a TruncatedError for what.
func (r *reader) take(n int, what string) []byte {
	if r.err != nil {
		return nil
	}
	if left := len(r.b) - r.off; n < 0 || n > left {
		r.fail(&TruncatedError{Offset: r.pos(), Need: max(n-left, 1), What: what}) // n < 0: a u4 length past int on 32-bit
		return nil
	}
	s := r.b[r.off : r.off+n]
	r.off += n
	return s
}

func (r *reader) u1(what string) uint8 {
	if s := r.take(1, what); s != nil {
		return s[0]
	}
	return 0
}

func (r *reader) u2(what string) uint16 {
	if s := r.take(2, what); s != nil {
		return binary.BigEndian.Uint16(s)
	}
	return 0
}

func (r *reader) u4(what string) uint32 {
	if s := r.take(4, what); s != nil {
		return binary.BigEndian.Uint32(s)
	}
	return 0
}
