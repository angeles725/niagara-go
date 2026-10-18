package help

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/angeles725/niagara-go/internal/cli"
	"github.com/angeles725/niagara-go/internal/golden"
	"github.com/angeles725/niagara-go/internal/pyre"
)

// TestGoldenReal runs the niagara-help keep-set cases of the Python golden suite
// (niagara-help tests/golden_cli.py) against the real checkout named by
// NIAGARA_HELP_DIR. testdata/golden-real holds what the frozen niagara_help.py
// printed, normalized as that harness normalizes it (scratch dir -> <TMP>,
// checkout -> <REPO>, elapsed times -> <T>); "{tmp}" in args is a fresh scratch
// directory. The Go output is normalized the same way before comparing. With
// GOLDEN_BIN set, each case runs that binary instead of Main. docs/goldens.md
// ("Real-corpus goldens") says how to re-capture.
//
// The runner is local because internal/golden compares raw bytes and has no
// placeholder for a checkout path or the harness's time normalization.
func TestGoldenReal(t *testing.T) {
	repo := os.Getenv(EnvDir)
	if repo == "" {
		t.Skip("NIAGARA_HELP_DIR is not set: the real-corpus goldens need the niagara-help main checkout")
	}
	repo, err := filepath.Abs(repo)
	if err != nil {
		t.Fatal(err)
	}
	if st, err := os.Stat(filepath.Join(repo, "indexes")); err != nil || !st.IsDir() {
		t.Fatalf("NIAGARA_HELP_DIR=%s has no indexes/ directory", repo)
	}
	cases := loadRealCases(t, filepath.Join("testdata", "golden-real"))
	bin := realGoldenBin(t)
	defer func(f func() (string, error)) { executable = f }(executable)
	executable = func() (string, error) { return "", nil } // the checkout is found from the cwd
	times := pyre.MustCompile(`\d+(\.\d+)?\s?(ms|s|sec|secs|seconds)\b`, 0)

	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) {
			if c.Intended != nil {
				t.Logf("intended difference: %s", c.Intended.Reason)
			}
			tmp := t.TempDir()
			args := make([]string, len(c.Args))
			for i, a := range c.Args {
				args[i] = strings.ReplaceAll(a, golden.TmpPlaceholder, tmp)
			}
			// The harness runs with COLUMNS=100 and without the NIAGARA_HELP_* variables.
			env := map[string]string{"COLUMNS": "100", EnvDir: "", envVersionForTest: ""}
			out, errb, code := runRealCase(t, bin, repo, args, env)
			norm := func(b []byte) []byte {
				s := strings.ReplaceAll(string(b), tmp, "<TMP>")
				s = strings.ReplaceAll(s, repo, "<REPO>")
				return []byte(pySub(times, s, "<T>"))
			}
			compareReal(t, c, norm(out), norm(errb), code)
		})
	}
}

// loadRealCases reads every case directory under dir with golden.Load, which
// expects <testdata>/golden/<case>: a scratch testdata links golden -> dir.
func loadRealCases(t *testing.T, dir string) []golden.Case {
	t.Helper()
	abs, err := filepath.Abs(dir)
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	if err := os.Symlink(abs, filepath.Join(root, "golden")); err != nil {
		t.Fatal(err)
	}
	cases, err := golden.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	return cases
}

// realGoldenBin is GOLDEN_BIN as an absolute path (a bare name keeps its PATH
// lookup), or "" to run Main in process.
func realGoldenBin(t *testing.T) string {
	bin := os.Getenv(golden.BinEnv)
	if strings.ContainsRune(bin, '/') || strings.ContainsRune(bin, os.PathSeparator) {
		abs, err := filepath.Abs(bin)
		if err != nil {
			t.Fatal(err)
		}
		return abs
	}
	return bin
}

// runRealCase runs args in dir with env (an empty value unsets the variable),
// in process or through bin, with empty stdin.
func runRealCase(t *testing.T, bin, dir string, args []string, env map[string]string) (stdout, stderr []byte, exit int) {
	t.Helper()
	var out, errb bytes.Buffer
	if bin == "" {
		for k, v := range env {
			t.Setenv(k, v)
		}
		t.Chdir(dir)
		code := Main(args, cli.Stdio{In: bytes.NewReader(nil), Out: &out, Err: &errb})
		return out.Bytes(), errb.Bytes(), code
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Dir = dir
	for _, kv := range os.Environ() {
		if k, _, _ := strings.Cut(kv, "="); !hasKey(env, k) {
			cmd.Env = append(cmd.Env, kv)
		}
	}
	for k, v := range env {
		if v != "" {
			cmd.Env = append(cmd.Env, k+"="+v)
		}
	}
	cmd.Stdin = bytes.NewReader(nil)
	cmd.Stdout, cmd.Stderr = &out, &errb
	cmd.WaitDelay = time.Second
	err := cmd.Run()
	if ctx.Err() != nil {
		t.Fatalf("run %s: no exit after 10m", bin)
	}
	var exitErr *exec.ExitError
	switch {
	case errors.As(err, &exitErr):
		return out.Bytes(), errb.Bytes(), exitErr.ExitCode()
	case err != nil:
		t.Fatalf("run %s: %v", bin, err)
	}
	return out.Bytes(), errb.Bytes(), 0
}

func hasKey(m map[string]string, k string) bool { _, ok := m[k]; return ok }

// pySub is re.sub(rx, repl, s) for a literal repl.
func pySub(rx *pyre.Regexp, s, repl string) string {
	text := []rune(s)
	var b strings.Builder
	last := 0
	for _, m := range rx.FindIter(text) {
		b.WriteString(string(text[last:m.Start()]))
		b.WriteString(repl)
		last = m.End()
	}
	b.WriteString(string(text[last:]))
	return b.String()
}

func compareReal(t *testing.T, c golden.Case, stdout, stderr []byte, exit int) {
	t.Helper()
	if exit != c.Exit {
		t.Errorf("exit = %d, want %d", exit, c.Exit)
	}
	if !bytes.Equal(stdout, c.Stdout) {
		t.Errorf("stdout differs:\n%s", golden.Diff(c.Stdout, stdout))
	}
	if !bytes.Equal(stderr, c.Stderr) {
		t.Errorf("stderr differs:\n%s", golden.Diff(c.Stderr, stderr))
	}
}
