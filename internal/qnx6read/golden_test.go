package qnx6read

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/angeles725/niagara-go/internal/cli"
)

// runDir holds the sparse disk images of testdata/inputs/images.txt; golden cases run there.
var runDir string

func TestMain(m *testing.M) {
	code, err := withImages(m)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		code = 2
	}
	os.Exit(code)
}

func withImages(m *testing.M) (int, error) {
	dir, err := os.MkdirTemp("", "qnx6read-golden-")
	if err != nil {
		return 0, err
	}
	defer os.RemoveAll(dir)
	if err := materializeImages(dir); err != nil {
		return 0, err
	}
	runDir = dir
	return m.Run(), nil
}

// materializeImages writes each manifest image as a sparse file: zeros up to the partition
// offset, then the committed partition bytes (the real offsets are 129 MiB and 3.5 GiB).
func materializeImages(dir string) error {
	f, err := os.Open(filepath.Join("testdata", "inputs", "images.txt"))
	if err != nil {
		return err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		var img, part string
		var off int64
		if _, err := fmt.Sscan(line, &img, &off, &part); err != nil {
			return fmt.Errorf("images.txt %q: %v", line, err)
		}
		data, err := os.ReadFile(part)
		if err != nil {
			return err
		}
		dst := filepath.Join(dir, img)
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return err
		}
		out, err := os.Create(dst)
		if err != nil {
			return err
		}
		_, werr := out.WriteAt(data, off)
		if cerr := out.Close(); werr == nil {
			werr = cerr
		}
		if werr != nil {
			return werr
		}
	}
	return sc.Err()
}

type goldenCase struct {
	name           string
	args           []string
	env            map[string]string
	stdout, stderr []byte
	exit           int
}

func loadGolden(t *testing.T) []goldenCase {
	t.Helper()
	dirs, err := filepath.Glob(filepath.Join("testdata", "golden", "*"))
	if err != nil || len(dirs) == 0 {
		t.Fatalf("no golden cases (%v)", err)
	}
	var cases []goldenCase
	for _, d := range dirs {
		var cmd struct {
			Args []string          `json:"args"`
			Env  map[string]string `json:"env"`
		}
		c := goldenCase{name: filepath.Base(d)}
		raw, err := os.ReadFile(filepath.Join(d, "cmd.json"))
		if err == nil {
			err = json.Unmarshal(raw, &cmd)
		}
		if err == nil {
			c.stdout, err = os.ReadFile(filepath.Join(d, "stdout"))
		}
		if err == nil {
			c.stderr, err = os.ReadFile(filepath.Join(d, "stderr"))
		}
		var code []byte
		if err == nil {
			code, err = os.ReadFile(filepath.Join(d, "exit"))
		}
		if err == nil {
			c.exit, err = strconv.Atoi(strings.TrimSpace(string(code)))
		}
		if err != nil {
			t.Fatalf("%s: %v", d, err)
		}
		c.args, c.env = cmd.Args, cmd.Env
		cases = append(cases, c)
	}
	return cases
}

// optionsFromArgs maps the argv of a non-usage golden case onto Options (a test fixture, not a
// parser: usage and error text belong to internal/argparse).
func optionsFromArgs(t *testing.T, c goldenCase) Options {
	t.Helper()
	o := Options{Image: c.env["QNX6_IMG"], Part: "P2"}
	var pos []string
	for i := 0; i < len(c.args); i++ {
		if c.args[i] == "--image" && i+1 < len(c.args) {
			o.Image = c.args[i+1]
			i++
			continue
		}
		pos = append(pos, c.args[i])
	}
	if len(pos) > 0 {
		o.Part, pos = pos[0], pos[1:]
	}
	if len(pos) == 3 && pos[0] == "extract" {
		o.Extract = &Extract{Src: pos[1], Dst: pos[2]}
	} else if len(pos) != 0 {
		t.Fatalf("golden argv %q is not a plain case; name it usage-*", c.args)
	}
	return o
}

// intended lists the cases where Python ends in a traceback; Go prints one line instead
// (stdout still has to match the golden).
var intended = map[string]struct {
	exit   int
	stderr string
}{
	"extract-missing": {1, "P2 OFF=3 blocksize=1024 inodes=12\nqnx6read: /etc/nope: not found in the image\n"},
	"extract-bad-dst": {2, "P2 OFF=3 blocksize=1024 inodes=12\n" +
		"qnx6read: cannot write no-such-dir/out.bin: No such file or directory\n"},
	"not-qnx6": {2, "qnx6read: not a QNX6 partition at byte 135266304 of testdata/images/not-qnx6.img " +
		"(superblock magic 0x12345678)\n"},
	"list-p3-image-wrong-partition": {2, "qnx6read: not a QNX6 partition at byte 135266304 of " +
		"testdata/images/p3.img (superblock magic 0x00000000)\n"},
	"no-offset": {2, "qnx6read: could not detect block offset in testdata/images/no-offset.img\n"},
	"tiny-image": {2, "qnx6read: not a QNX6 partition at byte 135266304 of testdata/images/tiny.img " +
		"(superblock truncated: 0 of 4096 bytes)\n"},
}

// extracted is the sha256 of each extract case's output file (the Python run produced the same).
var extracted = map[string]string{
	"out-hosts.bin":  "081ef9d5367595d16e30b4b4549d9f43537320508b4ce0788963e10e4f808857",
	"out-big.bin":    "79377de5e174f4d17e4bc9550a5776175255f1bc0c3f2cdbb118c61b7e301d22",
	"out-huge.bin":   "8027a9995b59bb9a9d2fb0954230758215fceab214c41b21d3366943acc922b4",
	"out-long.bin":   "98752ee28d5484bdc2814fb70adb6a0b2fb31f6a9b8ee7ae81fd2fc9cf300b3b",
	"out-cafe.bin":   "9a6af4d867277033f9f282bdaac4dc7a2c148f1fd6bf4446955517fa44be1134",
	"out-readme.bin": "646db9abafe4f1ece93f8bc80f98968cc8fa443df7d4de5457ad3b73c85eb337",
	"out-zero.bin":   "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
	"out-etc.bin":    "67b50ddbe8d5cf46f80cc857836367d5e445748845bdcf69b218dc0f2ad5304e",
	"out-root.bin":   "671bcd4339a4d5f52e3aa8c84becf0616b6ef5fee8df280b91b5026d002cb69a",
}

func TestGolden(t *testing.T) {
	cases := loadGolden(t)
	t.Chdir(runDir)
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if strings.HasPrefix(c.name, "usage-") {
				t.Skip("awaiting internal/argparse")
			}
			o := optionsFromArgs(t, c)
			var out, errb bytes.Buffer
			code := Run(o, cli.Stdio{In: strings.NewReader(""), Out: &out, Err: &errb})
			wantExit, wantErr := c.exit, string(c.stderr)
			if d, ok := intended[c.name]; ok {
				wantExit, wantErr = d.exit, d.stderr
			}
			if code != wantExit {
				t.Errorf("exit = %d, want %d", code, wantExit)
			}
			if !bytes.Equal(out.Bytes(), c.stdout) {
				t.Errorf("stdout differs\n got: %q\nwant: %q", out.String(), c.stdout)
			}
			if errb.String() != wantErr {
				t.Errorf("stderr differs\n got: %q\nwant: %q", errb.String(), wantErr)
			}
			if o.Extract != nil && code == 0 {
				data, err := os.ReadFile(o.Extract.Dst)
				sum := sha256.Sum256(data)
				if err != nil || hex.EncodeToString(sum[:]) != extracted[o.Extract.Dst] {
					t.Errorf("%s: sha256 %x (%v), want %s", o.Extract.Dst, sum, err, extracted[o.Extract.Dst])
				}
			}
		})
	}
}
