package pycompat

import "fmt"

// B64Decode is the lenient (validate=False) core of base64.b64decode, or of
// base64.urlsafe_b64decode when urlsafe is set, over bytes: every byte outside
// the alphabet ('=' and non-ASCII included) is skipped, and a trailing 2- or
// 3-character group decodes to its whole bytes. The urlsafe alphabet maps '-'
// and '_' to 62 and 63 and still accepts '+' and '/', as Python's translate
// step does; the standard alphabet skips '-' and '_'. Only a lone trailing
// character is an error, with binascii's message and no exception prefix.
//
// Two Python checks are left to the caller: the ASCII-only check a str
// argument gets, and, for the standard alphabet, CPython 3.15's "Incorrect
// padding" on an unpadded trailing group, which this decoder accepts.
func B64Decode(s string, urlsafe bool) ([]byte, error) {
	var out []byte
	quad, left := 0, 0
	for i := 0; i < len(s); i++ {
		v := b64Value(s[i], urlsafe)
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
		//lint:ignore ST1005 parity with the Python message
		return nil, fmt.Errorf("Invalid base64-encoded string: number of data characters (%d) cannot be 1 more than a multiple of 4", len(out)/3*4+1)
	}
	return out, nil
}

// b64Value is the 6-bit value of c in the chosen alphabet, or -1 to skip it.
func b64Value(c byte, urlsafe bool) int {
	switch {
	case c >= 'A' && c <= 'Z':
		return int(c - 'A')
	case c >= 'a' && c <= 'z':
		return int(c-'a') + 26
	case c >= '0' && c <= '9':
		return int(c-'0') + 52
	case c == '+' || urlsafe && c == '-':
		return 62
	case c == '/' || urlsafe && c == '_':
		return 63
	}
	return -1
}
