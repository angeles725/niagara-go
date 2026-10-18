package qnx6read

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"

	"github.com/angeles725/niagara-go/internal/golden"
)

// root is a fresh directory per test process, so concurrent `go test` runs of this package never
// share files: root/golden links to (or, without symlink privilege, copies) testdata/golden,
// root/linux/golden to testdata/linux/golden (TestGoldenLinux), and root/run (the cwd_fixture of every case)
// holds the sparse disk images of testdata/inputs/images.txt (the real P2/P3 offsets are 129 MiB
// and 3.5 GiB) and the extract outputs. TestMain removes it afterwards.
var root, runDir string

func TestMain(m *testing.M) {
	code, err := withImages(m)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		code = 2
	}
	os.Exit(code)
}

func withImages(m *testing.M) (int, error) {
	testdata, err := filepath.Abs("testdata")
	if err != nil {
		return 0, err
	}
	dir, err := os.MkdirTemp("", "qnx6read-golden-")
	if err != nil {
		return 0, err
	}
	defer os.RemoveAll(dir)
	if err := linkOrCopy(filepath.Join(testdata, "golden"), filepath.Join(dir, "golden"), os.Symlink); err != nil {
		return 0, err
	}
	if err := os.Mkdir(filepath.Join(dir, "linux"), 0o755); err != nil {
		return 0, err
	}
	if err := linkOrCopy(filepath.Join(testdata, "linux", "golden"), filepath.Join(dir, "linux", "golden"), os.Symlink); err != nil {
		return 0, err
	}
	if err := materializeImages(testdata, dir); err != nil {
		return 0, err
	}
	root, runDir = dir, filepath.Join(dir, "run")
	return m.Run(), nil
}

// linkOrCopy links dst to the directory src, or copies the tree when symlink fails (Windows
// without symlink privilege).
func linkOrCopy(src, dst string, symlink func(oldname, newname string) error) error {
	if symlink(src, dst) == nil {
		return nil
	}
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		out := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(out, 0o755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(out, data, 0o644)
	})
}

// materializeImages writes each manifest image under dst as a sparse file: zeros up to the
// partition offset, then the committed partition bytes. Image paths are relative to dst, partition
// paths to testdata.
func materializeImages(testdata, dst string) error {
	f, err := os.Open(filepath.Join(testdata, "inputs", "images.txt"))
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
		data, err := os.ReadFile(filepath.Join(testdata, part))
		if err != nil {
			return err
		}
		path := filepath.Join(dst, img)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		out, err := os.Create(path)
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

// TestGolden runs every case through Main (intended differences come from each case's
// intended.json), then checks the bytes every successful extract case wrote.
func TestGolden(t *testing.T) {
	golden.Run(t, root, Main)
	cases, err := golden.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, c := range cases {
		if len(c.Args) < 3 || c.Args[len(c.Args)-3] != "extract" || c.Exit != 0 {
			continue
		}
		dst := c.Args[len(c.Args)-1]
		seen[dst] = true
		want, ok := extracted[dst]
		if !ok {
			t.Errorf("%s: no sha256 recorded for %s", c.Name, dst)
			continue
		}
		data, err := os.ReadFile(filepath.Join(runDir, dst))
		sum := sha256.Sum256(data)
		if err != nil || hex.EncodeToString(sum[:]) != want {
			t.Errorf("%s: %s sha256 %x (%v), want %s", c.Name, dst, sum, err, want)
		}
	}
	for _, dst := range unmatchedHashes(extracted, seen) {
		t.Errorf("extracted[%q] is checked by no successful extract case", dst)
	}
}

// TestGoldenLinux runs the cases that need a Linux device node (/dev/null is a character
// device there; Python on Windows finds no such file and says "image not found").
func TestGoldenLinux(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("captured on Linux: /dev/null is a character device there")
	}
	golden.Run(t, filepath.Join(root, "linux"), Main)
}

// unmatchedHashes returns, sorted, the keys of hashes absent from seen.
func unmatchedHashes(hashes map[string]string, seen map[string]bool) []string {
	var out []string
	for dst := range hashes {
		if !seen[dst] {
			out = append(out, dst)
		}
	}
	sort.Strings(out)
	return out
}

// Without symlink privilege (Windows) linkOrCopy copies the golden tree instead of aborting.
func TestLinkOrCopyFallsBackToCopy(t *testing.T) {
	src, dst := t.TempDir(), filepath.Join(t.TempDir(), "golden")
	if err := os.MkdirAll(filepath.Join(src, "case"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "case", "cmd.json"), []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	noLink := func(string, string) error { return &os.LinkError{Op: "symlink", Err: os.ErrPermission} }
	if err := linkOrCopy(src, dst, noLink); err != nil {
		t.Fatal(err)
	}
	if fi, err := os.Lstat(dst); err != nil || fi.Mode()&os.ModeSymlink != 0 || !fi.IsDir() {
		t.Fatalf("dst %v (%v): want a copied directory", fi, err)
	}
	if b, err := os.ReadFile(filepath.Join(dst, "case", "cmd.json")); err != nil || string(b) != "{}\n" {
		t.Fatalf("copied cmd.json = %q, %v", b, err)
	}
}

// unmatchedHashes names every extracted entry no extract case checked.
func TestUnmatchedHashes(t *testing.T) {
	got := unmatchedHashes(map[string]string{"a": "1", "b": "2", "c": "3"}, map[string]bool{"b": true})
	if strings.Join(got, ",") != "a,c" {
		t.Fatalf("unmatched = %q, want a,c", got)
	}
}
