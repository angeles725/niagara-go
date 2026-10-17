package pycompat

import (
	"strings"
	"unicode/utf8"

	"github.com/angeles725/niagara-go/internal/textfold"
)

// Strip is str.strip() with no argument.
func Strip(s string) string { return strings.TrimFunc(s, IsSpace) }

// Fields is str.split() with no separator; it is textfold.Fields.
func Fields(s string) []string { return textfold.Fields(s) }

// LineBreakLen returns the byte length of the str.splitlines() boundary at s[i:] ("\r\n" is
// one two-byte boundary), or 0 when s[i:] does not start with one.
func LineBreakLen(s string, i int) int {
	switch c := s[i]; c {
	case '\r':
		if i+1 < len(s) && s[i+1] == '\n' {
			return 2
		}
		return 1
	case '\n', '\v', '\f', 0x1c, 0x1d, 0x1e:
		return 1
	case 0xc2:
		if i+1 < len(s) && s[i+1] == 0x85 {
			return 2
		}
	case 0xe2:
		if i+2 < len(s) && s[i+1] == 0x80 && (s[i+2] == 0xa8 || s[i+2] == 0xa9) {
			return 3
		}
	}
	return 0
}

// SplitLines is str.splitlines(keepends).
func SplitLines(s string, keepends bool) []string {
	lines := []string{}
	start := 0
	for i := 0; i < len(s); {
		n := LineBreakLen(s, i)
		if n == 0 {
			i++
			continue
		}
		if keepends {
			lines = append(lines, s[start:i+n])
		} else {
			lines = append(lines, s[start:i])
		}
		i += n
		start = i
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}

// UniversalNewlines is what reading a text file opened with newline=None (the default)
// returns: "\r\n" and a lone "\r" become "\n"; no other line boundary is touched.
func UniversalNewlines(s string) string {
	if !strings.ContainsRune(s, '\r') {
		return s
	}
	return strings.ReplaceAll(strings.ReplaceAll(s, "\r\n", "\n"), "\r", "\n")
}

// Lines is list(f) of such a file: UniversalNewlines(s) cut after each "\n", ends kept.
// Unlike SplitLines, "\v", "\f", "\x1c", "\x85", U+2028 and U+2029 are not boundaries.
func Lines(s string) []string {
	s = UniversalNewlines(s)
	lines := []string{}
	for s != "" {
		i := strings.IndexByte(s, '\n') + 1
		if i == 0 {
			i = len(s)
		}
		lines = append(lines, s[:i])
		s = s[i:]
	}
	return lines
}

// DecodeReplace is bytes.decode("utf-8", "replace"): one U+FFFD per maximal subpart of an
// ill-formed sequence (Go's own decoder yields one per byte).
func DecodeReplace(b []byte) string {
	if utf8.Valid(b) {
		return string(b)
	}
	var sb strings.Builder
	sb.Grow(len(b) + 16)
	for i := 0; i < len(b); {
		c := b[i]
		if c < utf8.RuneSelf {
			sb.WriteByte(c)
			i++
			continue
		}
		need, lo, hi := seqShape(c)
		j := i + 1
		for k := 0; k < need && j < len(b); k++ {
			if b[j] < lo || b[j] > hi {
				break
			}
			j++
			lo, hi = 0x80, 0xbf
		}
		if need > 0 && j-i == need+1 {
			sb.Write(b[i:j])
		} else {
			sb.WriteRune(utf8.RuneError)
		}
		i = j
	}
	return sb.String()
}

// seqShape returns how many continuation bytes lead byte c needs and the valid range of the
// first one; need is 0 for a byte that cannot start a sequence.
func seqShape(c byte) (need int, lo, hi byte) {
	switch {
	case c >= 0xc2 && c <= 0xdf:
		return 1, 0x80, 0xbf
	case c == 0xe0:
		return 2, 0xa0, 0xbf
	case c == 0xed:
		return 2, 0x80, 0x9f
	case c >= 0xe1 && c <= 0xef:
		return 2, 0x80, 0xbf
	case c == 0xf0:
		return 3, 0x90, 0xbf
	case c >= 0xf1 && c <= 0xf3:
		return 3, 0x80, 0xbf
	case c == 0xf4:
		return 3, 0x80, 0x8f
	}
	return 0, 0, 0
}

// PosixBasename is posixpath.basename.
func PosixBasename(p string) string { return p[strings.LastIndex(p, "/")+1:] }

// PosixSplitext is posixpath.splitext: (root, ext), leading dots of the name are not an ext.
func PosixSplitext(p string) (string, string) {
	sep, dot := strings.LastIndex(p, "/"), strings.LastIndex(p, ".")
	if dot > sep {
		for i := sep + 1; i < dot; i++ {
			if p[i] != '.' {
				return p[:dot], p[dot:]
			}
		}
	}
	return p, ""
}

// PosixJoin is posixpath.join: an absolute part restarts the path; nothing is normalised.
func PosixJoin(first string, rest ...string) string {
	out := first
	for _, p := range rest {
		switch {
		case strings.HasPrefix(p, "/"):
			out = p
		case out == "" || strings.HasSuffix(out, "/"):
			out += p
		default:
			out += "/" + p
		}
	}
	return out
}
