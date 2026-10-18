package cli

import "testing"

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
