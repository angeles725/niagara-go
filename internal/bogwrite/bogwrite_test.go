package bogwrite

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/angeles725/niagara-go/internal/cli"
	"github.com/angeles725/niagara-go/internal/golden"
)

// TestGolden replays the cases frozen from tools/bog-write.py by the retired testdata/capture.py.
func TestGolden(t *testing.T) { golden.Run(t, "testdata", Main) }

func run(args ...string) (int, string, string) {
	var out, errb bytes.Buffer
	code := Main(args, cli.Stdio{Out: &out, Err: &errb})
	return code, out.String(), errb.String()
}

func mustRead(t *testing.T, p string) []byte {
	t.Helper()
	data, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// TestGenerated replays testdata/generated: generate in a scratch directory, then compare
// the output with Python's and the written file with the one Python wrote.
func TestGenerated(t *testing.T) {
	td, err := filepath.Abs("testdata")
	if err != nil {
		t.Fatal(err)
	}
	dirs, err := filepath.Glob(filepath.Join(td, "generated", "*"))
	if err != nil || len(dirs) == 0 {
		t.Fatalf("no generated cases: %v", err)
	}
	for _, d := range dirs {
		t.Run(filepath.Base(d), func(t *testing.T) {
			var cmd struct {
				Args []string `json:"args"`
			}
			if err := json.Unmarshal(mustRead(t, filepath.Join(d, "cmd.json")), &cmd); err != nil {
				t.Fatal(err)
			}
			if len(cmd.Args) < 2 {
				t.Fatalf("cmd.json args %q: want generate <plan> ...", cmd.Args)
			}
			tmp := t.TempDir()
			plan := cmd.Args[1]
			if err := os.WriteFile(filepath.Join(tmp, plan), mustRead(t, filepath.Join(td, "fixtures", plan)), 0o644); err != nil {
				t.Fatal(err)
			}
			t.Chdir(tmp)
			start := time.Now()
			code, out, errText := run(cmd.Args...)
			want, err := strconv.Atoi(strings.TrimSpace(string(mustRead(t, filepath.Join(d, "exit")))))
			if err != nil {
				t.Fatal(err)
			}
			if code != want || out != string(mustRead(t, filepath.Join(d, "stdout"))) || errText != string(mustRead(t, filepath.Join(d, "stderr"))) {
				t.Fatalf("exit %d out %q err %q", code, out, errText)
			}
			if _, err := os.Stat(filepath.Join(d, "out.xml")); err == nil {
				if got := mustRead(t, filepath.Join(tmp, "out.xml")); !bytes.Equal(got, mustRead(t, filepath.Join(d, "out.xml"))) {
					t.Errorf("out.xml differs:\n%s", got)
				}
				return
			}
			checkZip(t, filepath.Join(tmp, "out.bog"), filepath.Join(d, "out.bog"), mustRead(t, filepath.Join(d, "file.xml")), start)
		})
	}
}

// checkZip compares the ZIP Go wrote with Python's: one deflated file.xml with the same
// header fields and content. The deflate bytes and the timestamp (local time now) differ.
// The reference was written on Linux (version made by 3 = Unix); Windows CPython's ZipInfo
// sets create_system 0 (MS-DOS), so on Windows the expected creator is 0<<8 | 20.
func checkZip(t *testing.T, gotPath, wantPath string, xml []byte, start time.Time) {
	t.Helper()
	got, err := zip.OpenReader(gotPath)
	if err != nil {
		t.Fatal(err)
	}
	defer got.Close()
	want, err := zip.OpenReader(wantPath)
	if err != nil {
		t.Fatal(err)
	}
	defer want.Close()
	if len(got.File) != 1 || len(want.File) != 1 {
		t.Fatalf("entries: got %d want %d", len(got.File), len(want.File))
	}
	g, w := got.File[0].FileHeader, want.File[0].FileHeader
	if w.CreatorVersion != 3<<8|20 {
		t.Fatalf("reference version made by %#x, want Unix 2.0 (0x314)", w.CreatorVersion)
	}
	if runtime.GOOS == "windows" {
		w.CreatorVersion = 0<<8 | 20
	}
	if g.Name != w.Name || g.Method != w.Method || g.Flags != w.Flags || g.CreatorVersion != w.CreatorVersion ||
		g.ReaderVersion != w.ReaderVersion || g.ExternalAttrs != w.ExternalAttrs || !bytes.Equal(g.Extra, w.Extra) ||
		g.CRC32 != w.CRC32 || g.UncompressedSize64 != w.UncompressedSize64 {
		t.Errorf("header:\n got  %+v\n want %+v", g, w)
	}
	r, err := got.File[0].Open()
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if _, err := buf.ReadFrom(r); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(buf.Bytes(), xml) {
		t.Errorf("file.xml differs:\n%s", buf.Bytes())
	}
	m := g.Modified // MS-DOS time has no zone: it is the local wall clock
	if m = time.Date(m.Year(), m.Month(), m.Day(), m.Hour(), m.Minute(), m.Second(), 0, time.Local); m.Before(start.Add(-3*time.Second)) || m.After(time.Now().Add(time.Second)) {
		t.Errorf("timestamp %v is not the local time now", m)
	}
}

var selftestOut = regexp.MustCompile(`^[.s]+\n-{70}\nRan (\d+) tests in \d+\.\d{3}s\n\nOK( \(skipped=1\))?\n$`)

func TestSelftestReportsLikeUnittest(t *testing.T) {
	code, out, errText := run("selftest")
	m := selftestOut.FindStringSubmatch(errText)
	if code != 0 || out != "" || m == nil || m[1] == "0" {
		t.Errorf("exit %d out %q err %q", code, out, errText)
	}
}
