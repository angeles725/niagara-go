package pxrender

import (
	"bytes"
	"testing"

	"github.com/angeles725/niagara-go/internal/cli"
)

// runMain runs Main in testdata/ with env as the whole environment and returns stdout,
// stderr and the exit code.
func runMain(t *testing.T, env map[string]string, args ...string) (string, string, int) {
	t.Helper()
	var out, errb bytes.Buffer
	code := Main(args, cli.Stdio{Out: &out, Err: &errb, Getenv: func(name string) string { return env[name] }})
	return out.String(), errb.String(), code
}

// --organized defaults to $NIAGARA_ORGANIZED, so a PATH wrapper can pass the root; the flag
// wins over the variable, and with neither (or the variable empty) no organized/ tree is read.
func TestOrganizedFromEnv(t *testing.T) {
	t.Chdir("testdata")
	px := []string{"inputs/station/shared/px/main.px", "--shared", "inputs/station/shared"}
	flag := append(append([]string{}, px...), "--organized", "inputs/organized")

	want, _, code := runMain(t, nil, flag...)
	if code != 0 {
		t.Fatalf("--organized run exited %d", code)
	}
	none, _, _ := runMain(t, nil, px...)
	if none == want {
		t.Fatal("fixture does not exercise organized/: output is the same without it")
	}

	cases := []struct {
		name string
		env  map[string]string
		args []string
		want string
	}{
		{"env only", map[string]string{"NIAGARA_ORGANIZED": "inputs/organized"}, px, want},
		{"flag wins over env", map[string]string{"NIAGARA_ORGANIZED": "inputs/missing"}, flag, want},
		{"empty env is none", map[string]string{"NIAGARA_ORGANIZED": ""}, px, none},
		{"neither", nil, px, none},
	}
	for _, c := range cases {
		got, stderr, code := runMain(t, c.env, c.args...)
		if code != 0 || got != c.want {
			t.Errorf("%s: exit %d, stdout matches want: %v; stderr %q", c.name, code, got == c.want, stderr)
		}
	}
}
