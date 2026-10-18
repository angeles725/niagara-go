package helpbuild

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// Unit tests of pyfs.go and registry.go: the Python semantics the S3/S4
// builders rely on (the parity tests cover them end to end).

// writeTree writes files (relative "/"-separated path -> content) under root.
func writeTree(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for rel, text := range files {
		writeFile(t, filepath.Join(root, filepath.FromSlash(rel)), []byte(text))
	}
}

// pyWalk lists a symlinked folder among the folders without entering it,
// a broken symlink among the files, and descends only what visit returns.
func TestPyWalk(t *testing.T) {
	top := t.TempDir()
	writeTree(t, top, map[string]string{"a/x": "", "a/skip/y": "", "f": "", "real/z": ""})
	if err := os.Symlink(filepath.Join(top, "real"), filepath.Join(top, "a", "link")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(top, "missing"), filepath.Join(top, "broken")); err != nil {
		t.Fatal(err)
	}
	var got []string
	pyWalk(top, func(dir string, dirs, files []string) []string {
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
		t.Errorf("pyWalk:\n got %q\nwant %q", got, want)
	}
}

func TestRemoveFileRefusesADirectory(t *testing.T) {
	dir := t.TempDir()
	d := filepath.Join(dir, "x.txt")
	if err := os.Mkdir(d, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := removeFile(d); err == nil || !isDir(d) {
		t.Errorf("removeFile(dir) = %v, dir kept %v", err, isDir(d))
	}
	f := filepath.Join(dir, "f.txt")
	writeFile(t, f, nil)
	if err := removeFile(f); err != nil || isRegular(f) {
		t.Errorf("removeFile(file) = %v", err)
	}
}

func TestPyCopyfile(t *testing.T) {
	dir := t.TempDir()
	src, dst := filepath.Join(dir, "src"), filepath.Join(dir, "dst")
	writeFile(t, src, []byte("data"))
	writeFile(t, dst, []byte("longer old content"))
	if err := pyCopyfile(src, dst); err != nil || string(readBytes(t, dst)) != "data" {
		t.Fatalf("pyCopyfile: %v", err)
	}
	mkfifo, err := exec.LookPath("mkfifo")
	if err != nil {
		t.Skip("no mkfifo")
	}
	fifo := filepath.Join(dir, "fifo")
	if out, err := exec.Command(mkfifo, fifo).CombinedOutput(); err != nil {
		t.Fatalf("mkfifo: %v %s", err, out)
	}
	if err := pyCopyfile(fifo, dst); err == nil || err.Error() != "`"+fifo+"` is a named pipe" {
		t.Errorf("pyCopyfile(fifo) = %v", err)
	}
}

// loadJSON errors carry Python's str(exception); what Python catches is a
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
		writeFile(t, p, []byte(data))
		_, err := loadJSON(p)
		var le *JSONLoadError
		if !errors.As(err, &le) || le.OS || le.Msg != want {
			t.Errorf("loadJSON(%q) = %v, want %q", data, err, want)
		}
	}
	_, err := loadJSON(filepath.Join(dir, "missing.json"))
	var le *JSONLoadError
	if !errors.As(err, &le) || !le.OS || !strings.HasPrefix(le.Msg, "[Errno 2] No such file or directory: '") {
		t.Errorf("missing file: %v", err)
	}
}

// identify, installDir and courseDir skip what Python catches and fail on
// what makes it raise.
func TestRegistryLookups(t *testing.T) {
	dir := t.TempDir()
	repo := filepath.Join(dir, "repo")
	reg := func(text string) {
		writeFile(t, filepath.Join(repo, "versions", "registry.json"), []byte(text))
	}
	root := filepath.Join(repo, "versions", "A")
	reg(`{"versions": {"bad": 5, "nodr": {"install_dir": "/x"}, "num": {"data_root": 3},` +
		` "A": {"data_root": "versions/./A/", "install_dir": "/inst", "course_dir": "../c"}}}`)
	if vid, err := identify(root); vid != "A" || err != nil {
		t.Errorf("identify = %q, %v", vid, err)
	}
	if d, err := installDir(root); d != "/inst" || err != nil {
		t.Errorf("installDir = %q, %v", d, err)
	}
	if d, err := courseDir(root); d != filepath.Join(dir, "c") || err != nil {
		t.Errorf("courseDir = %q, %v", d, err)
	}
	for text, wantErr := range map[string]bool{
		`{"versions": {"A": {"data_root": "versions/A", "install_dir": ""}}}`: false,
		`{"versions": {"A": {"data_root": "versions/A", "install_dir": 0}}}`:  false,
		`{"versions": {"A": {"data_root": "versions/A", "install_dir": 5}}}`:  true,
		`{"versions": {"A": {"data_root": "versions/A", "course_dir": [1]}}}`: true,
		`{"versions": ["A"]}`: true, // identify skips it; course_dir's .get raises
		`{"versions": 5}`:     true,
		`[1]`:                 true,
		`{}`:                  false,
		`{"versions": null}`:  true,
	} {
		reg(text)
		_, err1 := installDir(root)
		_, err2 := courseDir(root)
		if got := err1 != nil || err2 != nil; got != wantErr {
			t.Errorf("%s: installDir %v, courseDir %v; want an error %v", text, err1, err2, wantErr)
		}
	}
}
