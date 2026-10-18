package nav

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
	"github.com/angeles725/niagara-go/internal/pycompat"
	"github.com/angeles725/niagara-go/internal/pyre"
)

// envNavDir names the module-navigator checkout the real-corpus goldens run against.
const envNavDir = "MODULE_NAVIGATOR_DIR"

// navNormalize is module-navigator tests/golden_cli.py _NORMALIZE, applied to
// stdout and stderr; navNormalizeStderr is _NORMALIZE_STDERR, applied after it
// to stderr only.
var (
	navNormalize = []navRule{
		{pyre.MustCompile(`\b\d+(?:\.\d+)?\s?(?:ms|s|sec|seconds)\b`, 0), "<T>"},
		{pyre.MustCompile(`\b\d{4}-\d{2}-\d{2}[T ]\d{2}:\d{2}(?::\d{2}(?:\.\d+)?)?`, 0), "<DATE>"},
		{pyre.MustCompile(`\b\d{8}[-_]\d{6}\b`, 0), "<STAMP>"},
	}
	navNormalizeStderr = []navRule{
		{pyre.MustCompile(`\(\d+(?:\.\d+)? MB\)`, 0), "(<MB>)"},
		{pyre.MustCompile(`^WARNING: .*(?:was built from|corpus modules had|rebuild with \./reindex\.sh).*\n`, pyre.MULTILINE), ""},
	}
)

type navRule struct {
	rx   *pyre.Regexp
	repl string
}

// TestGoldenReal runs the module-navigator keep-set cases of the Python golden
// suite (module-navigator tests/golden_cli.py) against the real checkout named
// by MODULE_NAVIGATOR_DIR and its organized/ corpus. testdata/golden-real holds
// what the frozen module_nav.py printed, normalized as that harness normalizes
// it. Each case runs as the harness runs it: cwd the checkout, COLUMNS=100,
// "{tmp}" (the --base-dir value) a fresh scratch directory whose indexes/ links
// to the checkout's, inside a run directory holding a niagara-help link when
// the sibling checkout exists. The Go output is normalized the same way before
// comparing. With GOLDEN_BIN set, each case runs that binary instead of Main.
// docs/goldens.md ("Real-corpus goldens") says how to re-capture.
//
// The runner is local because internal/golden compares raw bytes and has no
// placeholder for a checkout path or the harness's normalization.
func TestGoldenReal(t *testing.T) {
	nav := os.Getenv(envNavDir)
	if nav == "" {
		t.Skip("MODULE_NAVIGATOR_DIR is not set: the real-corpus goldens need the module-navigator checkout and its organized/ corpus")
	}
	nav, err := filepath.Abs(nav)
	if err != nil {
		t.Fatal(err)
	}
	if st, err := os.Stat(filepath.Join(nav, "indexes")); err != nil || !st.IsDir() {
		t.Fatalf("MODULE_NAVIGATOR_DIR=%s has no indexes/ directory", nav)
	}
	corpora := navCorpusDirs(nav)
	if st, err := os.Stat(corpora[0]); err != nil || !st.IsDir() {
		t.Fatalf("organized/ corpus %s not found (set NAV_CORPUS_BASE or put it next to the checkout)", corpora[0])
	}
	cases := loadRealCases(t, filepath.Join("testdata", "golden-real"))
	bin := realGoldenBin(t)
	run := t.TempDir()
	if help := filepath.Join(filepath.Dir(nav), "niagara-help"); pycompat.IsDir(help) {
		if err := os.Symlink(help, filepath.Join(run, "niagara-help")); err != nil {
			t.Fatal(err)
		}
	}

	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) {
			if c.Intended != nil {
				t.Logf("intended difference: %s", c.Intended.Reason)
			}
			base, err := os.MkdirTemp(run, "base-")
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { os.RemoveAll(base) })
			if err := os.Symlink(filepath.Join(nav, "indexes"), filepath.Join(base, "indexes")); err != nil {
				t.Fatal(err)
			}
			args := make([]string, len(c.Args))
			for i, a := range c.Args {
				args[i] = strings.ReplaceAll(a, golden.TmpPlaceholder, base)
			}
			out, errb, code := runRealCase(t, bin, nav, args, map[string]string{"COLUMNS": "100"})
			norm := func(b []byte, rules []navRule) []byte {
				s := strings.ReplaceAll(string(b), base, "<BASE>")
				s = strings.ReplaceAll(s, run, "<RUN>")
				for _, corpus := range corpora {
					s = pySub(corpusPath(corpus), s, "<CORPUS>")
				}
				s = strings.ReplaceAll(s, nav, "<NAV>")
				for _, r := range append(append([]navRule{}, navNormalize...), rules...) {
					s = pySub(r.rx, s, r.repl)
				}
				return []byte(s)
			}
			compareReal(t, c, norm(out, nil), norm(errb, navNormalizeStderr), code)
		})
	}
}

// navCorpusDirs is the harness's corpus_dirs(): the organized/ paths a command
// may print, the one in use first ($NAV_CORPUS_BASE when set, as
// core.OrganizedDir), then the default sibling of the checkout.
func navCorpusDirs(nav string) []string {
	var dirs []string
	for _, d := range []string{os.Getenv("NAV_CORPUS_BASE"), filepath.Join(filepath.Dir(nav), "organized")} {
		if d = strings.TrimRight(d, string(os.PathSeparator)); d != "" {
			dirs = append(dirs, d)
		}
	}
	return dirs
}

// corpusPath matches corpus as a whole path: not followed by a path character
// (a "." only ends a sentence), as the harness's normalize().
func corpusPath(corpus string) *pyre.Regexp {
	var esc strings.Builder
	for _, r := range corpus {
		if r < 128 && !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '_') {
			esc.WriteByte('\\')
		}
		esc.WriteRune(r)
	}
	return pyre.MustCompile(esc.String()+`(?![^\s/'"),:;.]|\.[^\s'"),:;])`, 0)
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

// runRealCase runs args in dir with env added, in process or through bin,
// with empty stdin.
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
	cmd.Env = os.Environ()
	for k, v := range env {
		cmd.Env = append(cmd.Env, k+"="+v)
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
