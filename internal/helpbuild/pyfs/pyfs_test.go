package pyfs

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/angeles725/niagara-go/internal/helpbuild/internal/testkit"
	"github.com/angeles725/niagara-go/internal/pycompat"
)

// Unit tests of pyfs.go: the Python semantics the S3/S4
// builders rely on (the parity tests cover them end to end).

// Walk lists a symlinked folder among the folders without entering it,
// a broken symlink among the files, and descends only what visit returns.
func TestPyWalk(t *testing.T) {
	top := t.TempDir()
	testkit.WriteTree(t, top, map[string]string{"a/x": "", "a/skip/y": "", "f": "", "real/z": ""})
	testkit.SymlinkOrSkip(t, filepath.Join(top, "real"), filepath.Join(top, "a", "link"))
	testkit.SymlinkOrSkip(t, filepath.Join(top, "missing"), filepath.Join(top, "broken"))
	var got []string
	Walk(top, func(dir string, dirs, files []string) []string {
		rel, _ := filepath.Rel(top, dir)
		got = append(got, rel+" "+strings.Join(dirs, ",")+" | "+strings.Join(files, ","))
		var keep []string
		for _, d := range dirs {
			if d != "skip" {
				keep = append(keep, d)
			}
		}
		return keep
	})
	want := []string{". a,real | broken,f", "a link,skip | x", "real  | z"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Walk:\n got %q\nwant %q", got, want)
	}
}

func TestRemoveFileRefusesADirectory(t *testing.T) {
	dir := t.TempDir()
	d := filepath.Join(dir, "x.txt")
	if err := os.Mkdir(d, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := RemoveFile(d); err == nil || !pycompat.IsDir(d) {
		t.Errorf("RemoveFile(dir) = %v, dir kept %v", err, pycompat.IsDir(d))
	}
	f := filepath.Join(dir, "f.txt")
	testkit.WriteFile(t, f, nil)
	if err := RemoveFile(f); err != nil || pycompat.IsFile(f) {
		t.Errorf("RemoveFile(file) = %v", err)
	}
}

func TestPyCopyfile(t *testing.T) {
	dir := t.TempDir()
	src, dst := filepath.Join(dir, "src"), filepath.Join(dir, "dst")
	testkit.WriteFile(t, src, []byte("data"))
	testkit.WriteFile(t, dst, []byte("longer old content"))
	if err := Copyfile(src, dst); err != nil || string(testkit.ReadBytes(t, dst)) != "data" {
		t.Fatalf("Copyfile: %v", err)
	}
	mkfifo, err := exec.LookPath("mkfifo")
	if err != nil {
		t.Skip("no mkfifo")
	}
	fifo := filepath.Join(dir, "fifo")
	if out, err := exec.Command(mkfifo, fifo).CombinedOutput(); err != nil {
		t.Fatalf("mkfifo: %v %s", err, out)
	}
	if err := Copyfile(fifo, dst); err == nil || err.Error() != "`"+fifo+"` is a named pipe" {
		t.Errorf("Copyfile(fifo) = %v", err)
	}
}

// shutil.copyfile raises SameFileError before opening dst when both names
// are one file (the same name, a symlink either way, a hard link), so the
// file is never truncated; a directory source fails with
// IsADirectoryError before dst is opened, so dst keeps its content.
func TestPyCopyfileRefusesSameFileAndDirectory(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "a")
	testkit.WriteFile(t, a, []byte("data"))
	link, hard := filepath.Join(dir, "link"), filepath.Join(dir, "hard")
	testkit.SymlinkOrSkip(t, a, link)
	if err := os.Link(a, hard); err != nil {
		t.Fatal(err)
	}
	for _, p := range [][2]string{{a, a}, {a, link}, {link, a}, {hard, a}} {
		err := Copyfile(p[0], p[1])
		want := pycompat.FSRepr(p[0]) + " and " + pycompat.FSRepr(p[1]) + " are the same file"
		if err == nil || pycompat.OSErrorText(err) != want {
			t.Errorf("Copyfile(%s, %s) = %v, want %q", p[0], p[1], err, want)
		}
		if got := string(testkit.ReadBytes(t, a)); got != "data" {
			t.Fatalf("Copyfile(%s, %s) left %q", p[0], p[1], got)
		}
	}
	src, dst := filepath.Join(dir, "d"), filepath.Join(dir, "dst")
	if err := os.Mkdir(src, 0o755); err != nil {
		t.Fatal(err)
	}
	testkit.WriteFile(t, dst, []byte("keep"))
	err := Copyfile(src, dst)
	if want := "[Errno 21] Is a directory: " + pycompat.FSRepr(src); err == nil || pycompat.OSErrorText(err) != want {
		t.Errorf("Copyfile(dir, dst) = %v, want %q", err, want)
	}
	if got := string(testkit.ReadBytes(t, dst)); got != "keep" {
		t.Errorf("Copyfile(dir, dst) truncated dst to %q", got)
	}
	err = Copyfile(a, src) // a directory as destination: open(dst, "wb") fails
	if want := "[Errno 21] Is a directory: " + pycompat.FSRepr(src); err == nil || pycompat.OSErrorText(err) != want {
		t.Errorf("Copyfile(a, dir) = %v, want %q", err, want)
	}
}

// LoadJSON errors carry Python's str(exception); what Python catches is a
// *JSONLoadError.
func TestLoadJSONErrors(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "x.json")
	for data, want := range map[string]string{
		"{":           "Expecting property name enclosed in double quotes: line 1 column 2 (char 1)",
		"\r\n\r\n}":   "Expecting value: line 3 column 1 (char 2)", // universal newlines first
		"\xff{}":      "'utf-8' codec can't decode byte 0xff in position 0: invalid start byte",
		"\ufeff{}":    "Unexpected UTF-8 BOM (decode using utf-8-sig): line 1 column 1 (char 0)",
		`{"a": 1} {}`: "Extra data: line 1 column 10 (char 9)",
	} {
		testkit.WriteFile(t, p, []byte(data))
		_, err := LoadJSON(p)
		var le *JSONLoadError
		if !errors.As(err, &le) || le.OS || le.Msg != want {
			t.Errorf("LoadJSON(%q) = %v, want %q", data, err, want)
		}
	}
	_, err := LoadJSON(filepath.Join(dir, "missing.json"))
	var le *JSONLoadError
	if !errors.As(err, &le) || !le.OS || !strings.HasPrefix(le.Msg, "[Errno 2] No such file or directory: '") {
		t.Errorf("missing file: %v", err)
	}
}
