package rankcmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/angeles725/niagara-go/internal/cli"
	"github.com/angeles725/niagara-go/internal/nav/core"
	"github.com/angeles725/niagara-go/internal/nav/registry"
)

// skipsIndex is the rank-skips golden fixture: a mirror copy, an inner class,
// a null module and two modules of one name.
func skipsIndex(t *testing.T) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "testdata", "fixture", "rank-skips", "indexes", classIndex))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// newBase writes indexes/<name> files under a temporary base directory.
func newBase(t *testing.T, files map[string][]byte) string {
	t.Helper()
	base := t.TempDir()
	if err := os.MkdirAll(filepath.Join(base, "indexes"), 0o755); err != nil {
		t.Fatal(err)
	}
	for name, data := range files {
		if err := os.WriteFile(filepath.Join(base, "indexes", name), data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return base
}

func envFor(base string) *registry.Env {
	var out, errb bytes.Buffer
	return &registry.Env{Stdio: cli.Stdio{Out: &out, Err: &errb}, Runtime: core.NewAt("", base, &out, &errb)}
}

func run(t *testing.T, base, question string, limit int) (stdout, stderr string, code int) {
	t.Helper()
	t.Setenv("NAV_CORPUS_BASE", filepath.Join(base, "no-corpus"))
	env := envFor(base)
	Cmd(env, question, limit)
	return env.Out.(*bytes.Buffer).String(), env.Err.(*bytes.Buffer).String(), env.ExitCode()
}

// TestRankErrorPaths pins the stderr and exit code of class_rank.py (captured
// from Python on the same files) when an index is missing or malformed.
func TestRankErrorPaths(t *testing.T) {
	cases := []struct {
		name   string
		files  map[string][]byte
		stdout string
		stderr string
		code   int
	}{
		{"no xref index", map[string][]byte{classIndex: skipsIndex(t)},
			"\n  Rank 'alarm': 3 matching classes (terms: alarm)\n" +
				"  Classes named in the question first, then BM25 x importer prior; columns: rank, score, class, module, path\n\n" +
				"1\t0.229\tBAlarmService\talarm-rt\talarm/alarm-rt/BAlarmService.java\n" +
				"2\t0.226\tServiceAlarm\tNone\tNone\n3\t0.225\tBServiceAlarm\tb-rt\tNone\n\n",
			"NOTE: optional xref index not found ({base}/indexes/xref-index.json); the output omits " +
				"what it adds. Build it with ./reindex.sh\n", 0},
		{"no classes table", map[string][]byte{classIndex: []byte(`{"_meta":{},"classez":{}}`)}, "",
			"ERROR: class index has no 'classes' table: {base}/indexes/class-index.json\n  Run: ./reindex.sh\n", 2},
		{"xref table not an object", map[string][]byte{classIndex: skipsIndex(t),
			xrefIndex: []byte(`{"class_imported_by":[1]}`)}, "",
			"ERROR: xref index has no 'class_imported_by' table: {base}/indexes/xref-index.json\n  Run: ./reindex.sh\n", 2},
		{"no class index", nil, "",
			"ERROR: class index not found: {base}/indexes/class-index.json\n  Run: ./reindex.sh\n", 2},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			base := newBase(t, c.files)
			stdout, stderr, code := run(t, base, "alarm", 20)
			// {base}/indexes/<name> is os.path.join(base, "indexes", name): "\" on Windows.
			indexes := filepath.Join(base, "indexes") + string(filepath.Separator)
			if want := strings.ReplaceAll(c.stderr, "{base}/indexes/", indexes); stderr != want {
				t.Errorf("stderr = %q, want %q", stderr, want)
			}
			if stdout != c.stdout || code != c.code {
				t.Errorf("stdout = %q (exit %d), want %q (exit %d)", stdout, code, c.stdout, c.code)
			}
		})
	}
}

func TestRankRejectsNonPositiveLimit(t *testing.T) {
	_, stderr, code := run(t, t.TempDir(), "alarm", 0)
	if stderr != "ERROR: rank: the limit must be a positive integer, got 0\n" || code != 2 {
		t.Errorf("stderr = %q, exit %d", stderr, code)
	}
}
