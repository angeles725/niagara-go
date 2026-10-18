package nav

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/angeles725/niagara-go/internal/cli"
)

// TestCallgraphCacheKeepsOutput runs callers and callees on a base whose
// call graph index is past the cache floor and the 20 MB progress line
// (trailing JSON whitespace): the first run writes the cache, the second
// hits it, and both print the same stdout, stderr and exit code.
func TestCallgraphCacheKeepsOutput(t *testing.T) {
	t.Setenv("NAV_CORPUS_BASE", filepath.Join("testdata", "fixture", "no-corpus"))
	base := t.TempDir()
	if err := os.MkdirAll(filepath.Join(base, "indexes"), 0o755); err != nil {
		t.Fatal(err)
	}
	for name, pad := range map[string]int{"callgraph-index.json": 21 << 20, "class-index.json": 0} {
		b, err := os.ReadFile(filepath.Join("testdata", "fixture", "cg", "indexes", name))
		if err != nil {
			t.Fatal(err)
		}
		b = append(b, bytes.Repeat([]byte(" "), pad)...)
		if err := os.WriteFile(filepath.Join(base, "indexes", name), b, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	cache := filepath.Join(base, "indexes", ".mnav-cache", "callgraph.gob")
	for _, args := range [][]string{{"callers", "BTarget", "run"}, {"callers", "BTarget"}, {"callees", "BCaller"},
		{"callees", "BCaller", "work"}} {
		run := func() (string, string, int) {
			var out, errb bytes.Buffer
			code := Main(append([]string{"--base-dir", base}, args...), cli.Stdio{Out: &out, Err: &errb})
			return out.String(), errb.String(), code
		}
		os.Remove(cache)
		out1, err1, code1 := run()
		if _, err := os.Stat(cache); err != nil {
			t.Fatalf("%v: no cache written: %v", args, err)
		}
		out2, err2, code2 := run()
		if out1 != out2 || err1 != err2 || code1 != code2 {
			t.Errorf("%v: hit differs from miss:\n%q %q %d\n%q %q %d", args, out1, err1, code1, out2, err2, code2)
		}
		if !strings.HasPrefix(err1, "  Loading callgraph-index.json (21.0 MB)... OK\n") {
			t.Errorf("%v: stderr %q lacks the progress line", args, err1)
		}
	}
}
