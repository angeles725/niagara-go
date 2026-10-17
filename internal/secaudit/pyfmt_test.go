package secaudit

import (
	"errors"
	"os"
	"runtime"
	"syscall"
	"testing"

	"github.com/angeles725/niagara-go/internal/pyjson"
)

// The expectations below are what Python 3 prints for the same input (checked against CPython).

func TestJSONStringMatchesPython(t *testing.T) {
	for in, want := range map[string]string{
		"it's":                                 `"it's"`,
		`say "hi"`:                             `"say \"hi\""`,
		"tab\tnl\n\\":                          `"tab\tnl\n\\"`,
		"\x00\x7f\u0085\u200b\U0001F600\u00e9": `"\u0000\u007f\u0085\u200b\ud83d\ude00\u00e9"`,
		"\b\f\r":                               `"\b\f\r"`,
		"a\xffb":                               `"a\ufffdb"`, // not Python: see jsonStr
	} {
		if got := pyjson.Dumps(jsonStr(in), pyjson.DumpOpts{EnsureASCII: true, Indent: -1}); got != want {
			t.Errorf("json.dumps(%q) = %s, want %s", in, got, want)
		}
	}
}

func TestOSErrorTexts(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the expected texts are the Linux errno numbers and strerror messages")
	}
	_, err := os.Open("/nonexistent/x")
	if got, want := strerror(err), "No such file or directory"; got != want {
		t.Errorf("strerror = %q, want %q", got, want)
	}
	if got, want := strerror(syscall.EACCES), "Permission denied"; got != want {
		t.Errorf("strerror(EACCES) = %q, want %q", got, want)
	}
	if got := strerror(errors.New("plain")); got != "plain" {
		t.Errorf("strerror(non-OS error) = %q", got)
	}
}
