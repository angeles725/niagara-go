package bog

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// readExpectation is one entry of testdata/read/expected.json, written by the retired gen_fixtures.py from
// tools/lib/bogio.py: the BogReadError message ({path} = the input path) or the text's digest.
type readExpectation struct {
	Error  string `json:"error"`
	SHA256 string `json:"sha256"`
	Len    int    `json:"len"`
	Text   string `json:"text"`
}

func loadReadExpectations(t *testing.T) map[string]readExpectation {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "read", "expected.json"))
	if err != nil {
		t.Fatal(err)
	}
	var exp map[string]readExpectation
	if err := json.Unmarshal(data, &exp); err != nil {
		t.Fatal(err)
	}
	return exp
}

// detailDivergence lists the cases whose message differs from bogio.py only after the prefix:
// the detail text there comes from zlib, which Go's compress/flate does not reproduce.
var detailDivergence = map[string]string{
	"damaged-deflate.bog": "corrupt or unsupported ZIP file.xml of {path}: ",
}

// Every synthetic input reads to the same text, or fails with the same message, as bogio.py.
func TestReadTextMatchesBogio(t *testing.T) {
	for name, want := range loadReadExpectations(t) {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join("testdata", "read", name)
			text, err := ReadText(path)
			if want.Error != "" {
				wantMsg := strings.ReplaceAll(want.Error, "{path}", path)
				var re *ReadError
				if !errors.As(err, &re) {
					t.Fatalf("want *ReadError %q, got text of %d bytes, err %v", wantMsg, len(text), err)
				}
				if prefix, ok := detailDivergence[name]; ok {
					wantMsg = strings.ReplaceAll(prefix, "{path}", path)
					if !strings.HasPrefix(re.Error(), wantMsg) {
						t.Fatalf("message %q lacks the bogio prefix %q", re.Error(), wantMsg)
					}
					return
				}
				if re.Error() != wantMsg {
					t.Fatalf("message\n got %q\nwant %q", re.Error(), wantMsg)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			sum := sha256.Sum256([]byte(text))
			if got := hex.EncodeToString(sum[:]); got != want.SHA256 || len(text) != want.Len {
				t.Fatalf("text sha256 %s len %d, want %s len %d", got, len(text), want.SHA256, want.Len)
			}
			if want.Text != "" && text != want.Text {
				t.Fatalf("text\n got %q\nwant %q", text, want.Text)
			}
		})
	}
}

func TestReadBytesKeepsTheRawBytes(t *testing.T) {
	path := filepath.Join("testdata", "read", "utf8.bog")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	got, err := ReadBytes(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(raw) {
		t.Fatal("ReadBytes of a plain BOG must return the file bytes unchanged")
	}
}

// ExitTest.test_missing_file_fails and a directory: os.path.isfile is False for both.
func TestMissingFileAndDirectory(t *testing.T) {
	for _, path := range []string{filepath.Join(t.TempDir(), "nope.bog"), t.TempDir(), ""} {
		_, err := ReadText(path)
		var re *ReadError
		if !errors.As(err, &re) || re.Error() != "no such file: "+path {
			t.Fatalf("%q: got %v", path, err)
		}
	}
}

func TestUnreadableFileMessage(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("needs a non-root POSIX user")
	}
	path := filepath.Join(t.TempDir(), "noperm.bog")
	if err := os.WriteFile(path, []byte("x"), 0o000); err != nil {
		t.Fatal(err)
	}
	_, err := ReadText(path)
	want := "cannot read " + path + ": [Errno 13] Permission denied: '" + path + "'"
	if err == nil || err.Error() != want {
		t.Fatalf("got %v, want %q", err, want)
	}
}

// ReaderLimitsTest.test_oversized_member_is_refused_before_reading (MAX_MEMBER_BYTES patched to 64).
func TestOversizedMemberRefusedBeforeReading(t *testing.T) {
	old := memberLimit
	memberLimit = 64
	defer func() { memberLimit = old }()
	path := filepath.Join("testdata", "read", "ok.bog")
	_, err := ReadText(path)
	want := "file.xml of " + path + " is too large (607 bytes > 64)"
	if err == nil || err.Error() != want {
		t.Fatalf("got %v, want %q", err, want)
	}
}

// The ReadError keeps the archive library error for errors.Is/As; zip failures are never panics.
func TestReadErrorUnwrapsTheLibraryError(t *testing.T) {
	_, err := ReadText(filepath.Join("testdata", "read", "damaged.bog"))
	var re *ReadError
	if !errors.As(err, &re) || re.Unwrap() == nil {
		t.Fatalf("want a *ReadError wrapping the zip error, got %#v", err)
	}
}

func TestReadErrorExitCode(t *testing.T) {
	if ExitBadInput != 3 {
		t.Fatalf("ExitBadInput = %d, bog-nav pins 3 (D17)", ExitBadInput)
	}
}
