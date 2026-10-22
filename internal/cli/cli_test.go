package cli

import (
	"errors"
	"fmt"
	"testing"
)

func TestStdioEnvUsesTheInjectedGetenv(t *testing.T) {
	t.Parallel()
	s := Stdio{Getenv: func(name string) string { return "value of " + name }}
	if got := s.Env("HOME"); got != "value of HOME" {
		t.Errorf("Env = %q, want the injected value", got)
	}
}

func TestStdioEnvZeroValueReadsTheProcess(t *testing.T) {
	t.Setenv("CLI_STDIO_ENV_TEST", "from the process")
	if got := (Stdio{}).Env("CLI_STDIO_ENV_TEST"); got != "from the process" {
		t.Errorf("zero-value Env = %q, want the process value", got)
	}
}

func TestProcessStdioReadsTheProcessEnvironment(t *testing.T) {
	t.Setenv("CLI_STDIO_ENV_TEST", "seen")
	s := processStdio()
	if s.Getenv == nil || s.Env("CLI_STDIO_ENV_TEST") != "seen" {
		t.Errorf("processStdio Getenv is not os.Getenv")
	}
	if s.In == nil || s.Out == nil || s.Err == nil {
		t.Errorf("processStdio leaves a stream nil: %+v", s)
	}
}

func TestExitCodeFindsTheSystemExit(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name     string
		err      error
		wantMsg  string
		wantCode int
		wantOK   bool
	}{
		{"SystemExit(str) exits 1", SystemExit("ERROR: bad"), "ERROR: bad", 1, true},
		{"an explicit code", &Exit{Msg: "ERROR: none", Code: 2}, "ERROR: none", 2, true},
		{"wrapped", fmt.Errorf("ctx: %w", &Exit{Msg: "m", Code: 3}), "ctx: m", 3, true},
		{"any other error", errors.New("boom"), "boom", 0, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			code, ok := ExitCode(tc.err)
			if tc.err.Error() != tc.wantMsg || code != tc.wantCode || ok != tc.wantOK {
				t.Errorf("%q: ExitCode = %d, %v; want %q %d, %v", tc.err, code, ok, tc.wantMsg, tc.wantCode, tc.wantOK)
			}
		})
	}
}
