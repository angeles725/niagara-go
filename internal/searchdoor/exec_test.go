package searchdoor

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func python(t *testing.T) string {
	t.Helper()
	p, err := pythonInterpreter()
	if err != nil {
		t.Skip(err)
	}
	return p
}

func script(t *testing.T, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "tool.py")
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestExecBackend(t *testing.T) {
	py := python(t)
	b := execBackend{python: func() (string, error) { return py, nil }}
	t.Run("a missing tool is named", func(t *testing.T) {
		if got := b.Run([]string{"nope//x.py"}, 5); got.Reason != "unavailable: tool not found at nope/x.py" {
			t.Errorf("reason = %q", got.Reason)
		}
	})
	t.Run("universal newlines and replaced bytes", func(t *testing.T) {
		got := b.Run([]string{script(t, `import sys; sys.stdout.buffer.write(b"B1 \xc2\xb7 a\r\n  hit\xff\r\n")`)}, 5)
		if got.State != OK || got.Out != "B1 · a\n  hit�" {
			t.Errorf("raw = %+v", got)
		}
	})
	t.Run("arguments reach the tool", func(t *testing.T) {
		got := b.Run([]string{script(t, `import sys; print(" ".join(sys.argv[1:]))`), "find", "a b", "--and"}, 5)
		if got.Out != "find a b --and" {
			t.Errorf("out = %q", got.Out)
		}
	})
	t.Run("a timeout is typed unavailable", func(t *testing.T) {
		if testing.Short() {
			t.Skip("waits for the timeout")
		}
		got := b.Run([]string{script(t, "import time; time.sleep(10)")}, 0.3)
		if got.State != Unavailable || got.Reason != "unavailable: timed out after 0.3s" || got.HasRC {
			t.Errorf("raw = %+v", got)
		}
	})
	t.Run("no interpreter", func(t *testing.T) {
		nb := execBackend{python: func() (string, error) { return "", exec.ErrNotFound }}
		if got := nb.Run([]string{script(t, "")}, 1); !strings.HasPrefix(got.Reason, "unavailable: ") {
			t.Errorf("reason = %q", got.Reason)
		}
	})
}
