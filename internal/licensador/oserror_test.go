package licensador

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// TestOSErrorTexts pins str(OSError) (pycompat.OSErrorText) for the cases the key loaders and
// readers hit; the expectations are what Python 3 prints on Linux for open() on the same paths.
func TestOSErrorTexts(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the expected texts are the Linux errno numbers and strerror messages")
	}
	dir := t.TempDir()
	file := filepath.Join(dir, "f")
	if err := os.WriteFile(file, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	missing, below := filepath.Join(dir, "missing"), file+"/x"
	cases := []struct {
		name string
		err  error
		want string
	}{
		{"ENOENT", errOf(LoadPublicKey(missing)), "[Errno 2] No such file or directory: '" + missing + "'"},
		{"EISDIR", errOf(LoadPrivateKey(dir)), "[Errno 21] Is a directory: '" + dir + "'"},
		{"ENOTDIR", errOf(readASCII(below)), "[Errno 20] Not a directory: '" + below + "'"},
	}
	if os.Geteuid() != 0 {
		locked := filepath.Join(dir, "locked")
		if err := os.WriteFile(locked, nil, 0o000); err != nil {
			t.Fatal(err)
		}
		cases = append(cases, struct {
			name string
			err  error
			want string
		}{"EACCES", errOf(readASCII(locked)), "[Errno 13] Permission denied: '" + locked + "'"})
	}
	for _, tc := range cases {
		if tc.err == nil || tc.err.Error() != tc.want {
			t.Errorf("%s: got %v, want %q", tc.name, tc.err, tc.want)
		}
	}
}

func errOf[T any](_ T, err error) error { return err }
