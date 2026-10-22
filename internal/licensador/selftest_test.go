package licensador

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/angeles725/niagara-go/internal/cli"
)

// TestSelftestErrorExits pins how cmd_selftest fails before printing its
// matrix: each error surfaces as "error: <text>" on stderr with exit 2 and no
// table on stdout, whether it is returned (tmp, key size, blacklist write) or
// raised out of a verify (the state file cannot be written). The tmp-is-a-file
// text is the Go port's current one: Python's os.makedirs(exist_ok=True)
// raises "[Errno 17] File exists" there instead (a known divergence this test
// does not fix; it only keeps the routing stable).
func TestSelftestErrorExits(t *testing.T) {
	if testing.Short() {
		t.Skip("generates RSA keys")
	}
	if runtime.GOOS == "windows" {
		t.Skip("errno texts and paths are the POSIX ones")
	}
	cases := []struct {
		name  string
		bits  int
		setup func(t *testing.T, tmp string) string // returns the --tmp to pass
		err   string                                // stderr, %s = the --tmp passed
	}{
		{"tmp is a file", 1024, func(t *testing.T, tmp string) string {
			f := filepath.Join(tmp, "file")
			if err := os.WriteFile(f, nil, 0o666); err != nil {
				t.Fatal(err)
			}
			return f
		}, "error: [Errno 20] Not a directory: '%s'\n"},
		{"key too small", 512, func(t *testing.T, tmp string) string { return tmp },
			"error: key_size must be at least 1024-bits.\n%.0s"},
		{"blacklist unwritable", 1024, func(t *testing.T, tmp string) string {
			if err := os.Mkdir(filepath.Join(tmp, "bl.refl"), 0o777); err != nil {
				t.Fatal(err)
			}
			return tmp
		}, "error: [Errno 21] Is a directory: '%s/bl.refl'\n"},
		{"state unwritable", 1024, func(t *testing.T, tmp string) string {
			if err := os.Mkdir(filepath.Join(tmp, "state.json"), 0o777); err != nil {
				t.Fatal(err)
			}
			return tmp
		}, "error: [Errno 21] Is a directory: '%s/state.json'\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tmp := tc.setup(t, t.TempDir())
			var out, errb strings.Builder
			got := RunSelftest(tmp, tc.bits, cli.Stdio{In: strings.NewReader(""), Out: &out, Err: &errb})
			want := fmt.Sprintf(tc.err, tmp)
			if got != cli.ExitCannotSearch || out.String() != "" || errb.String() != want {
				t.Errorf("exit %d stdout %q stderr %q, want exit %d, no stdout, stderr %q",
					got, out.String(), errb.String(), cli.ExitCannotSearch, want)
			}
		})
	}
}
