package help

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// s4Case runs nh on the testdata/s4 checkout and compares stdout with
// testdata/misses/s4-<file>.txt, where <TESTDATA> stands for that checkout's
// absolute path (the S4 commands that print it: misses and stats).
func s4Case(t *testing.T, file string, wantCode int, args ...string) {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("testdata", "s4"))
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join("testdata", "misses", "s4-"+file+".txt"))
	if err != nil {
		t.Fatal(err)
	}
	want := strings.ReplaceAll(string(raw), "<TESTDATA>", root)
	out, errs, code := run(t, root, append([]string{"--niagara-version", "4.14.0.162"}, args...)...)
	if code != wantCode || out != want || errs != "" {
		t.Errorf("code %d stderr %q stdout:\n%s\nwant:\n%s", code, errs, out, want)
	}
}

func TestS4Misses(t *testing.T) {
	cases := []struct {
		file string
		args []string
	}{
		{"guide", []string{"guide", "zzqq"}},
		{"devguide", []string{"devguide", "zzqq"}},
	}
	for _, c := range cases {
		t.Run(c.file, func(t *testing.T) { s4Case(t, c.file, 1, c.args...) })
	}
}
