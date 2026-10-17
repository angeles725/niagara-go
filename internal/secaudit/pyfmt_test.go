package secaudit

import (
	"errors"
	"os"
	"reflect"
	"regexp"
	"syscall"
	"testing"
)

// The expectations below are what Python 3 prints for the same input (checked against CPython).

func TestPyReMatchesUnicodeWhitespace(t *testing.T) {
	re := regexp.MustCompile(pyRe(`^a\sb[\s,]c$`))
	for _, s := range []string{"a b c", "a\u00a0b,c", "a\u3000b\u2028c", "a\x1cb\x1fc"} {
		if !re.MatchString(s) {
			t.Errorf("%q: Python's \\s matches it", s)
		}
	}
	if re.MatchString("a\u200bb c") {
		t.Error("U+200B is not whitespace in Python")
	}
}

func TestPyStripAndIsPySpace(t *testing.T) {
	if got := pyStrip(" \x1c\x1fx\u00a0\u3000"); got != "x" {
		t.Errorf("pyStrip = %q, want \"x\"", got)
	}
	for _, r := range []rune{0x1c, 0x1d, 0x1e, 0x1f, 0x85, 0xa0} {
		if !isPySpace(r) {
			t.Errorf("isPySpace(%U) = false", r)
		}
	}
	if isPySpace(0x200b) {
		t.Error("isPySpace(U+200B) = true")
	}
}

func TestHeadCountsCodePoints(t *testing.T) {
	for _, tc := range []struct {
		in   string
		n    int
		want string
	}{{"a\u00f1ob", 2, "a\u00f1"}, {"ab", 5, "ab"}, {"ab", 0, ""}, {"\U0001f600x", 1, "\U0001f600"}} {
		if got := head(tc.in, tc.n); got != tc.want {
			t.Errorf("head(%q, %d) = %q, want %q", tc.in, tc.n, got, tc.want)
		}
	}
}

// decodeReplace replaces each maximal invalid subpart with one U+FFFD, as bytes.decode does.
func TestDecodeReplaceMatchesPython(t *testing.T) {
	for in, want := range map[string]string{
		"abc":              "abc",
		"\xff":             "\ufffd",
		"\xe2\x82":         "\ufffd",
		"\xe2\x82\xac":     "\u20ac",
		"\xed\xa0\x80":     "\ufffd\ufffd\ufffd", // an encoded surrogate
		"\xf0\x9f\x98":     "\ufffd",
		"\xf4\x90\x80\x80": "\ufffd\ufffd\ufffd\ufffd", // past U+10FFFF
		"a\xc0\xafb":       "a\ufffd\ufffdb",           // an overlong form
		"\xe0\x80\x80":     "\ufffd\ufffd\ufffd",
	} {
		if got := decodeReplace([]byte(in)); got != want {
			t.Errorf("decodeReplace(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestSplitLinesUniversalNewlines(t *testing.T) {
	if got, want := splitLines("a\r\nb\rc\nd"), []string{"a\n", "b\n", "c\n", "d"}; !reflect.DeepEqual(got, want) {
		t.Errorf("splitLines = %q, want %q", got, want)
	}
	if got := splitLines(""); len(got) != 0 {
		t.Errorf("splitLines(\"\") = %q", got)
	}
}

func TestPyJoin(t *testing.T) {
	for _, tc := range [][3]string{{"a", "b", "a/b"}, {"a/", "b", "a/b"}, {"", "b", "b"}} {
		if got := pyJoin(tc[0], tc[1]); got != tc[2] {
			t.Errorf("pyJoin(%q, %q) = %q, want %q", tc[0], tc[1], got, tc[2])
		}
	}
}

func TestPyStrRepr(t *testing.T) {
	for in, want := range map[string]string{
		"a\u00a0":                              `'a\xa0'`,
		"it's":                                 `"it's"`,
		`say "hi"`:                             `'say "hi"'`,
		`both '"`:                              `'both \'"'`,
		"tab\tnl\n\\":                          `'tab\tnl\n\\'`,
		"\x00\x7f\u0085\u200b\U0001F600\u00e9": `'\x00\x7f\x85\u200b` + "\U0001F600\u00e9'",
	} {
		if got := pyStrRepr(in); got != want {
			t.Errorf("pyStrRepr(%q) = %s, want %s", in, got, want)
		}
	}
	if got, want := pyListRepr([]string{"a", "b'c"}), `['a', "b'c"]`; got != want {
		t.Errorf("pyListRepr = %s, want %s", got, want)
	}
}

func TestPyJSONString(t *testing.T) {
	for in, want := range map[string]string{
		"it's":                                 `"it's"`,
		`say "hi"`:                             `"say \"hi\""`,
		"tab\tnl\n\\":                          `"tab\tnl\n\\"`,
		"\x00\x7f\u0085\u200b\U0001F600\u00e9": `"\u0000\u007f\u0085\u200b\ud83d\ude00\u00e9"`,
		"\b\f\r":                               `"\b\f\r"`,
	} {
		if got := pyJSONString(in); got != want {
			t.Errorf("pyJSONString(%q) = %s, want %s", in, got, want)
		}
	}
}

func TestOSErrorTexts(t *testing.T) {
	_, err := os.Open("/nonexistent/x")
	if got, want := pyOSError(err), "[Errno 2] No such file or directory: '/nonexistent/x'"; got != want {
		t.Errorf("pyOSError = %q, want %q", got, want)
	}
	if got, want := strerror(err), "No such file or directory"; got != want {
		t.Errorf("strerror = %q, want %q", got, want)
	}
	if got, want := strerror(syscall.EACCES), "Permission denied"; got != want {
		t.Errorf("strerror(EACCES) = %q, want %q", got, want)
	}
	if got := pyOSError(errors.New("plain")); got != "plain" {
		t.Errorf("pyOSError(non-OS error) = %q", got)
	}
}
