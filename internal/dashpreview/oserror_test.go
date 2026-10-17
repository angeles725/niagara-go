package dashpreview

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// TestLoadMockOSErrors pins the last traceback line of json.load(open(path)) for the OS
// errors a --mock path can hit; the expectations are what Python 3 prints on Linux.
func TestLoadMockOSErrors(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the expected texts are the Linux errno numbers and strerror messages")
	}
	dir := t.TempDir()
	file := filepath.Join(dir, "f.json")
	if err := os.WriteFile(file, []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	missing, below := filepath.Join(dir, "missing.json"), file+"/x"
	want := map[string]string{
		missing: "FileNotFoundError: [Errno 2] No such file or directory: '" + missing + "'",
		dir:     "IsADirectoryError: [Errno 21] Is a directory: '" + dir + "'",
		below:   "NotADirectoryError: [Errno 20] Not a directory: '" + below + "'",
	}
	if os.Geteuid() != 0 {
		locked := filepath.Join(dir, "locked.json")
		if err := os.WriteFile(locked, nil, 0o000); err != nil {
			t.Fatal(err)
		}
		want[locked] = "PermissionError: [Errno 13] Permission denied: '" + locked + "'"
	}
	for path, w := range want {
		if _, err := loadMock(path); err == nil || err.Error() != w {
			t.Errorf("loadMock(%q) = %v, want %q", path, err, w)
		}
	}
}
