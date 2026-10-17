package secaudit

import (
	"errors"
	"os"
	"reflect"
	"regexp"
	"runtime"
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

func TestSplitLinesUniversalNewlines(t *testing.T) {
	if got, want := splitLines("a\r\nb\rc\nd"), []string{"a\n", "b\n", "c\n", "d"}; !reflect.DeepEqual(got, want) {
		t.Errorf("splitLines = %q, want %q", got, want)
	}
	if got := splitLines(""); len(got) != 0 {
		t.Errorf("splitLines(\"\") = %q", got)
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
	if runtime.GOOS == "windows" {
		t.Skip("the expected texts are the Linux errno numbers and strerror messages")
	}
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
