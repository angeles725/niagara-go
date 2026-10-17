package pycompat

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

type ospathCase struct {
	path          string
	isDir, isFile bool
}

// The expectations are what os.path.isdir and os.path.isfile return for the same tree.
func TestIsDirIsFile(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "f")
	if err := os.WriteFile(file, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	cases := []ospathCase{
		{dir, true, false},
		{file, false, true},
		{filepath.Join(dir, "missing"), false, false},
		{filepath.Join(file, "below-a-file"), false, false}, // ENOTDIR
		{"a\x00b", false, false},                            // Python: ValueError -> False
	}
	if runtime.GOOS != "windows" {
		for _, l := range [][2]string{{"ld", dir}, {"lf", file}, {"lx", "missing"}} {
			if err := os.Symlink(l[1], filepath.Join(dir, l[0])); err != nil {
				t.Fatal(err)
			}
		}
		cases = append(cases,
			ospathCase{filepath.Join(dir, "ld"), true, false}, // symlinks are followed
			ospathCase{filepath.Join(dir, "lf"), false, true},
			ospathCase{filepath.Join(dir, "lx"), false, false}, // dangling
			ospathCase{"/dev/null", false, false})              // a device is not a regular file
	}
	for _, c := range cases {
		if got := IsDir(c.path); got != c.isDir {
			t.Errorf("IsDir(%q) = %v, want %v", c.path, got, c.isDir)
		}
		if got := IsFile(c.path); got != c.isFile {
			t.Errorf("IsFile(%q) = %v, want %v", c.path, got, c.isFile)
		}
	}
}
