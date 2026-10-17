package licensador

import (
	"errors"
	"fmt"
	"io/fs"
	"math/big"
	"strings"
	"syscall"

	"github.com/angeles725/niagara-go/internal/pycompat"
	"github.com/angeles725/niagara-go/internal/pyjson"
)

// Decoded text is WTF-8 (see internal/pyjson): Python str can hold lone
// surrogates, and decodeUTF8 with surrogatepass yields them.

// unicodeDecodeError renders Python's UnicodeDecodeError message.
func unicodeDecodeError(codec string, data []byte, start, end int, reason string) error {
	if end-start == 1 {
		return fmt.Errorf("'%s' codec can't decode byte 0x%02x in position %d: %s", codec, data[start], start, reason)
	}
	return fmt.Errorf("'%s' codec can't decode bytes in position %d-%d: %s", codec, start, end-1, reason)
}

// decodeASCII mirrors bytes.decode("ascii").
func decodeASCII(data []byte) (string, error) {
	for i, c := range data {
		if c >= 0x80 {
			return "", unicodeDecodeError("ascii", data, i, i+1, "ordinal not in range(128)")
		}
	}
	return string(data), nil
}

// decodeUTF8 mirrors bytes.decode("utf-8", errors) for errors "strict" or
// "surrogatepass" (CPython maximal-subpart error ranges).
func decodeUTF8(data []byte, surrogatepass bool) ([]rune, error) {
	out := make([]rune, 0, len(data))
	cont := func(i int, lo, hi byte) bool { return i < len(data) && data[i] >= lo && data[i] <= hi }
	for i := 0; i < len(data); {
		c := data[i]
		if c < 0x80 {
			out = append(out, rune(c))
			i++
			continue
		}
		var n int
		lo, hi := byte(0x80), byte(0xBF)
		switch {
		case c >= 0xC2 && c <= 0xDF:
			n = 2
		case c == 0xE0:
			n, lo = 3, 0xA0
		case c == 0xED:
			if surrogatepass && i+2 < len(data) && cont(i+1, 0xA0, 0xBF) && cont(i+2, 0x80, 0xBF) {
				out = append(out, rune(0xD000)|rune(data[i+1]&0x3F)<<6|rune(data[i+2]&0x3F))
				i += 3
				continue
			}
			n, hi = 3, 0x9F
		case c >= 0xE1 && c <= 0xEF:
			n = 3
		case c == 0xF0:
			n, lo = 4, 0x90
		case c >= 0xF1 && c <= 0xF3:
			n = 4
		case c == 0xF4:
			n, hi = 4, 0x8F
		default:
			return nil, unicodeDecodeError("utf-8", data, i, i+1, "invalid start byte")
		}
		r := rune(c) & (0x7F >> n)
		for k := 1; k < n; k++ {
			l, h := byte(0x80), byte(0xBF)
			if k == 1 {
				l, h = lo, hi
			}
			if i+k >= len(data) {
				return nil, unicodeDecodeError("utf-8", data, i, len(data), "unexpected end of data")
			}
			if !cont(i+k, l, h) {
				return nil, unicodeDecodeError("utf-8", data, i, i+k, "invalid continuation byte")
			}
			r = r<<6 | rune(data[i+k]&0x3F)
		}
		out = append(out, r)
		i += n
	}
	return out, nil
}

// decodeUTF16 mirrors bytes.decode("utf-16[-le|-be]", "surrogatepass").
// bom selects the "utf-16" codec (BOM consumed, endianness from it).
func decodeUTF16(data []byte, bigEndian, bom bool) ([]rune, error) {
	start := 0
	if bom {
		bigEndian = data[0] == 0xFE
		start = 2
	}
	codec := "utf-16-le"
	if bigEndian {
		codec = "utf-16-be"
	}
	unit := func(i int) rune {
		if bigEndian {
			return rune(data[i])<<8 | rune(data[i+1])
		}
		return rune(data[i+1])<<8 | rune(data[i])
	}
	var out []rune
	i := start
	for ; i+1 < len(data); i += 2 {
		u := unit(i)
		if u >= 0xD800 && u <= 0xDBFF && i+3 < len(data) {
			if l := unit(i + 2); l >= 0xDC00 && l <= 0xDFFF {
				out = append(out, 0x10000+(u-0xD800)<<10+(l-0xDC00))
				i += 2
				continue
			}
		}
		out = append(out, u)
	}
	if i < len(data) {
		return nil, unicodeDecodeError(codec, data, i, len(data), "truncated data")
	}
	return out, nil
}

// decodeUTF32 mirrors bytes.decode("utf-32[-le|-be]", "surrogatepass").
func decodeUTF32(data []byte, bigEndian, bom bool) ([]rune, error) {
	start := 0
	if bom {
		bigEndian = data[0] == 0x00
		start = 4
	}
	codec := "utf-32-le"
	if bigEndian {
		codec = "utf-32-be"
	}
	var out []rune
	i := start
	for ; i+3 < len(data); i += 4 {
		var u uint32
		if bigEndian {
			u = uint32(data[i])<<24 | uint32(data[i+1])<<16 | uint32(data[i+2])<<8 | uint32(data[i+3])
		} else {
			u = uint32(data[i+3])<<24 | uint32(data[i+2])<<16 | uint32(data[i+1])<<8 | uint32(data[i])
		}
		if u > 0x10FFFF {
			return nil, unicodeDecodeError(codec, data, i, i+4, "code point not in range(0x110000)")
		}
		out = append(out, rune(u))
	}
	if i < len(data) {
		return nil, unicodeDecodeError(codec, data, i, len(data), "truncated data")
	}
	return out, nil
}

// decodeJSONBytes mirrors json.loads(bytes): json.detect_encoding, then
// decode(..., "surrogatepass").
func decodeJSONBytes(b []byte) ([]rune, error) {
	switch {
	case len(b) >= 4 && (string(b[:4]) == "\x00\x00\xfe\xff" || string(b[:4]) == "\xff\xfe\x00\x00"):
		return decodeUTF32(b, false, true)
	case len(b) >= 2 && (string(b[:2]) == "\xfe\xff" || string(b[:2]) == "\xff\xfe"):
		return decodeUTF16(b, false, true)
	case len(b) >= 3 && string(b[:3]) == "\xef\xbb\xbf":
		return decodeUTF8(b[3:], true)
	case len(b) >= 4 && b[0] == 0:
		if b[1] != 0 {
			return decodeUTF16(b, true, false)
		}
		return decodeUTF32(b, true, false)
	case len(b) >= 4 && b[1] == 0:
		if b[2] != 0 || b[3] != 0 {
			return decodeUTF16(b, false, false)
		}
		return decodeUTF32(b, false, false)
	case len(b) == 2 && b[0] == 0:
		return decodeUTF16(b, true, false)
	case len(b) == 2 && b[1] == 0:
		return decodeUTF16(b, false, false)
	}
	return decodeUTF8(b, true)
}

// b64uDecode mirrors licensador._b64u_decode under Python 3.15: pad to a
// multiple of 4, then base64.urlsafe_b64decode in non-strict mode, where every
// non-alphabet byte ('=' included) is skipped and a trailing 2- or 3-character
// group decodes to its whole bytes; only a lone trailing character is an error.
func b64uDecode(s string) ([]byte, error) {
	for i := 0; i < len(s); i++ {
		if s[i] >= 0x80 {
			return nil, errors.New("string argument should contain only ASCII characters")
		}
	}
	s += strings.Repeat("=", (4-len(s)%4)%4)
	val := func(c byte) int {
		switch {
		case c >= 'A' && c <= 'Z':
			return int(c - 'A')
		case c >= 'a' && c <= 'z':
			return int(c-'a') + 26
		case c >= '0' && c <= '9':
			return int(c-'0') + 52
		case c == '-' || c == '+':
			return 62
		case c == '_' || c == '/':
			return 63
		}
		return -1
	}
	var out []byte
	quad, left := 0, 0
	for i := 0; i < len(s); i++ {
		v := val(s[i])
		if v < 0 {
			continue
		}
		switch quad {
		case 0:
			quad, left = 1, v
		case 1:
			out = append(out, byte(left<<2|v>>4))
			quad, left = 2, v&0x0F
		case 2:
			out = append(out, byte(left<<4|v>>2))
			quad, left = 3, v&0x03
		case 3:
			out = append(out, byte(left<<6|v))
			quad, left = 0, 0
		}
	}
	if quad == 1 {
		return nil, fmt.Errorf("Invalid base64-encoded string: number of data characters (%d) cannot be 1 more than a multiple of 4", len(out)/3*4+1)
	}
	return out, nil
}

// b64u mirrors licensador._b64u.
func b64u(data []byte) string {
	const alpha = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_"
	var sb strings.Builder
	for i := 0; i < len(data); i += 3 {
		var n uint32
		k := len(data) - i
		if k > 3 {
			k = 3
		}
		for j := 0; j < 3; j++ {
			n <<= 8
			if j < k {
				n |= uint32(data[i+j])
			}
		}
		for j := 0; j <= k; j++ {
			sb.WriteByte(alpha[n>>(18-6*j)&0x3F])
		}
	}
	return sb.String()
}

// pyOSError renders the str() of the OSError Python's open() raises.
func pyOSError(err error, name string) string {
	code, msg := 0, ""
	var errno syscall.Errno
	switch {
	case errors.Is(err, fs.ErrNotExist):
		code, msg = 2, "No such file or directory"
	case errors.Is(err, fs.ErrPermission):
		code, msg = 13, "Permission denied"
	case errors.As(err, &errno):
		code = int(errno)
		msg = errno.Error()
		if msg != "" {
			msg = strings.ToUpper(msg[:1]) + msg[1:]
		}
	default:
		return err.Error()
	}
	if errors.Is(err, syscall.EISDIR) {
		code, msg = 21, "Is a directory"
	}
	return fmt.Sprintf("[Errno %d] %s: %s", code, msg, pyjson.String(name).Repr())
}

// pyInt mirrors int(str) (pycompat.ParseBigInt) with CPython's ValueError text.
func pyInt(s string) (*big.Int, error) {
	n, err := pycompat.ParseBigInt(s)
	if err != nil {
		return nil, fmt.Errorf("invalid literal for int() with base 10: %s", pyjson.StrRepr(s))
	}
	return n, nil
}
