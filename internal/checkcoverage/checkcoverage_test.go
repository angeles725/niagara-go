package checkcoverage

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/angeles725/niagara-go/internal/cli"
	"github.com/angeles725/niagara-go/internal/golden"
)

// The cases are a subset of niagara-research tools/tests/fixtures/golden-cli/check-coverage.json
// (the Python output); the repository root is the case's working directory (testdata/corpus).

func useRoot(t *testing.T, root func() (string, error)) {
	t.Helper()
	saved := repoRoot
	repoRoot = root
	t.Cleanup(func() { repoRoot = saved })
}

func TestGolden(t *testing.T) {
	useRoot(t, os.Getwd)
	golden.Run(t, "testdata", Main)
}

// TestNoBlocksExits3 is check-coverage-empty.json: zero block files means the gate did
// not run, so "clear" would be a lie.
func TestNoBlocksExits3(t *testing.T) {
	root := t.TempDir()
	useRoot(t, func() (string, error) { return root, nil })
	for _, args := range [][]string{{"zzqq"}, {"--json", "zzqq"}} {
		var out, errb bytes.Buffer
		code := Main(args, cli.Stdio{In: strings.NewReader(""), Out: &out, Err: &errb})
		want := "check-coverage: no *bloque*.md block files under " + root + " -- gate not run\n"
		if code != 3 || out.Len() != 0 || errb.String() != want {
			t.Errorf("%v: exit %d, stdout %q, stderr %q", args, code, out.String(), errb.String())
		}
	}
}

func TestJSONString(t *testing.T) {
	if got := jsonString("a\"b\\c\n\x01ñ "); got != `"a\"b\\c\n\u0001ñ`+" \"" {
		t.Errorf("jsonString = %s", got)
	}
}
