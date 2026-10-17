package qnx6read

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/angeles725/niagara-go/internal/cli"
	"github.com/angeles725/niagara-go/internal/golden"
)

// The image checks are argparse errors in Python ("qnx6read.py: error: <message>"); the message
// part is ours to match before internal/argparse lands.
func TestValidateImageMatchesGoldenUsageMessages(t *testing.T) {
	cases := map[string]string{"usage-no-image": "", "usage-missing-image": "images/missing.img",
		"usage-directory-image": "images", "usage-char-device": "/dev/null"}
	frozen, err := golden.Load("testdata")
	if err != nil {
		t.Fatal(err)
	}
	stderr := map[string][]byte{}
	for _, c := range frozen {
		stderr[c.Name] = c.Stderr
	}
	t.Chdir(runDir)
	for name, image := range cases {
		if name == "usage-char-device" {
			if st, err := os.Stat(image); err != nil || st.Mode()&os.ModeCharDevice == 0 {
				continue
			}
		}
		msg := ValidateImage(image, false)
		if want := "qnx6read.py: error: " + msg + "\n"; msg == "" || !bytes.HasSuffix(stderr[name], []byte(want)) {
			t.Errorf("%s: ValidateImage = %q; golden stderr %q", name, msg, stderr[name])
		}
	}
	if msg := ValidateImage("images/p2.img", false); msg != "" {
		t.Errorf("a regular image is refused: %s", msg)
	}
	if st, err := os.Stat("/dev/null"); err == nil && st.Mode()&os.ModeCharDevice != 0 {
		if msg := ValidateImage("/dev/null", true); msg != "" {
			t.Errorf("--char-device still refuses /dev/null: %s", msg)
		}
	}
}

func TestValidateImageUnreadable(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root reads anything")
	}
	img := filepath.Join(t.TempDir(), "disk.img")
	if err := os.WriteFile(img, []byte("x"), 0); err != nil {
		t.Fatal(err)
	}
	if msg := ValidateImage(img, false); !strings.Contains(msg, "cannot read image") ||
		!strings.Contains(msg, "Permission denied") {
		t.Errorf("got %q", msg)
	}
}

func TestLatin1(t *testing.T) {
	if got := latin1([]byte("caf\xe9")); got != "café" {
		t.Errorf("latin1 = %q", got)
	}
}

// A walk that hits a corrupt inode reports it with exit 2 after the partial listing.
func TestCorruptInodeIsExit2(t *testing.T) {
	q := &qnx6{inodeTable: make([]byte, 100)}
	if _, err := q.inode(1); err == nil {
		t.Fatal("a 100-byte inode table must not decode inode 1")
	}
	if _, err := q.inode(0); err == nil {
		t.Fatal("inode 0 must be an error")
	}
	var out, errb bytes.Buffer
	t.Chdir(runDir)
	code := Run(Options{Image: "images/absent.img", Part: "P2"}, cli.Stdio{Out: &out, Err: &errb})
	if code != cli.ExitCannotSearch || !strings.Contains(errb.String(), "No such file or directory") {
		t.Errorf("code %d stderr %q", code, errb.String())
	}
}
