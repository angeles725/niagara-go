package golden

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/angeles725/niagara-go/internal/cli"
)

// BinEnv names the variable that switches Run to exec mode: when set, every
// case runs the binary at that path instead of the in-process Main.
const BinEnv = "GOLDEN_BIN"

// Run checks every case under testdata/golden against m (or GOLDEN_BIN).
// Cases run sequentially because they change the working directory and env.
func Run(t *testing.T, testdata string, m cli.Main) {
	t.Helper()
	testdata, err := filepath.Abs(testdata)
	if err != nil {
		t.Fatal(err)
	}
	cases, err := Load(testdata)
	if err != nil {
		t.Fatal(err)
	}
	bin := os.Getenv(BinEnv)
	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) {
			var got result
			if bin != "" {
				got = execCase(t, testdata, bin, c)
			} else {
				got = inProcess(t, testdata, m, c)
			}
			compare(t, c, got)
		})
	}
}

type result struct {
	stdout, stderr []byte
	exit           int
}

func workdir(testdata string, c Case) string {
	if c.CwdFixture == "" {
		return testdata
	}
	return filepath.Join(testdata, c.CwdFixture)
}

func stdin(t *testing.T, testdata string, c Case) *bytes.Reader {
	t.Helper()
	if c.StdinFile == "" {
		return bytes.NewReader(nil)
	}
	data, err := os.ReadFile(filepath.Join(testdata, c.StdinFile))
	if err != nil {
		t.Fatal(err)
	}
	return bytes.NewReader(data)
}

func inProcess(t *testing.T, testdata string, m cli.Main, c Case) result {
	for k, v := range c.Environ() {
		t.Setenv(k, v)
	}
	t.Chdir(workdir(testdata, c))
	var out, errb bytes.Buffer
	code := m(c.Args, cli.Stdio{In: stdin(t, testdata, c), Out: &out, Err: &errb})
	return result{out.Bytes(), errb.Bytes(), code}
}

func execCase(t *testing.T, testdata, bin string, c Case) result {
	cmd := exec.Command(bin, c.Args...)
	cmd.Dir = workdir(testdata, c)
	cmd.Env = os.Environ()
	for k, v := range c.Environ() {
		cmd.Env = append(cmd.Env, k+"="+v)
	}
	cmd.Stdin = stdin(t, testdata, c)
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	err := cmd.Run()
	code := 0
	var exitErr *exec.ExitError
	switch {
	case errors.As(err, &exitErr):
		code = exitErr.ExitCode()
	case err != nil:
		t.Fatalf("run %s: %v", bin, err)
	}
	return result{out.Bytes(), errb.Bytes(), code}
}

// reporter is the part of testing.T that compare needs (fakeable in tests).
type reporter interface {
	Helper()
	Errorf(format string, args ...any)
}

func compare(t *testing.T, c Case, got result) { compareInto(t, c, got) }

func compareInto(t reporter, c Case, got result) {
	t.Helper()
	if got.exit != c.Exit {
		t.Errorf("exit = %d, want %d", got.exit, c.Exit)
	}
	if !bytes.Equal(got.stdout, c.Stdout) {
		t.Errorf("stdout differs:\n%s", Diff(c.Stdout, got.stdout))
	}
	if !bytes.Equal(got.stderr, c.Stderr) {
		t.Errorf("stderr differs:\n%s", Diff(c.Stderr, got.stderr))
	}
}
