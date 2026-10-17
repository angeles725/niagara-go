package nlicense

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// TestReadFeaturesCSVOpenErrors pins the exception line Python prints when
// open(path, newline="", encoding="utf-8") fails in read_features_csv
// (captured from Python 3.15 on Linux).
func TestReadFeaturesCSVOpenErrors(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("errno numbers and texts are Linux's")
	}
	t.Chdir(t.TempDir())
	if err := os.Mkdir("adir", 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile("afile", []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile("noperm.csv", []byte("f,a=1\n"), 0o000); err != nil {
		t.Fatal(err)
	}
	cases := []struct{ path, want string }{
		{"nope.csv", "FileNotFoundError: [Errno 2] No such file or directory: 'nope.csv'"},
		{"it's.csv", `FileNotFoundError: [Errno 2] No such file or directory: "it's.csv"`},
		{"adir", "IsADirectoryError: [Errno 21] Is a directory: 'adir'"},
		{filepath.Join("afile", "sub.csv"), "NotADirectoryError: [Errno 20] Not a directory: 'afile/sub.csv'"},
	}
	if os.Geteuid() != 0 {
		cases = append(cases, struct{ path, want string }{"noperm.csv", "PermissionError: [Errno 13] Permission denied: 'noperm.csv'"})
	}
	for _, tc := range cases {
		_, err := readFeaturesCSV(tc.path)
		if err == nil || err.Error() != tc.want {
			t.Errorf("%s: %v, want %q", tc.path, err, tc.want)
		}
	}
}
