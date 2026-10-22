package pycompat

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// TestMakedirs pins os.makedirs(name, exist_ok) against the str(OSError) Python 3.15 prints
// on Linux; the paths are relative to a directory holding a regular file "f".
func TestMakedirs(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("errno texts are the Linux ones")
	}
	cases := []struct {
		name    string
		path    string
		existOK bool
		want    string // str(OSError), "" when makedirs returns
		dir     string // a directory that must exist afterwards
	}{
		{"new nested", "a/b/c", true, "", "a/b/c"},
		{"trailing separator", "n/m/", true, "", "n/m"},
		{"dot tail", "d/.", true, "", "d"},
		{"existing dir, exist_ok", ".", true, "", "."},
		{"existing dir, not exist_ok", "e", false, "[Errno 17] File exists: 'e'", ""},
		{"existing file", "f", true, "[Errno 17] File exists: 'f'", ""},
		{"existing file, trailing separator", "f/", true, "[Errno 17] File exists: 'f/'", ""},
		{"existing file, dot head", "./f", true, "[Errno 17] File exists: './f'", ""},
		{"file as parent", "f/sub", true, "[Errno 20] Not a directory: 'f/sub'", ""},
		{"file as grandparent", "f/a/b", true, "[Errno 20] Not a directory: 'f/a'", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Chdir(t.TempDir())
			if err := os.WriteFile("f", nil, 0o666); err != nil {
				t.Fatal(err)
			}
			if err := os.Mkdir("e", 0o777); err != nil {
				t.Fatal(err)
			}
			got := ""
			if err := Makedirs(tc.path, 0o777, tc.existOK); err != nil {
				got = OSErrorText(err)
			}
			if got != tc.want {
				t.Fatalf("Makedirs(%q, %v) = %q, want %q", tc.path, tc.existOK, got, tc.want)
			}
			if tc.dir != "" {
				if fi, err := os.Stat(filepath.FromSlash(tc.dir)); err != nil || !fi.IsDir() {
					t.Errorf("%s is not a directory afterwards: %v", tc.dir, err)
				}
			}
		})
	}
}
