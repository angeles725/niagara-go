package searchdoor

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"syscall"
	"time"
)

var processStart = time.Now()

// newDoor is the front door of the binaries: the in-process backends with exec
// behind them, or exec alone when $NSEARCH_EXEC is 1 (door.bind then gives the
// in-process backend the script paths it answers for).
func newDoor() *door {
	var b Backend = execBackend{python: pythonInterpreter}
	if os.Getenv(ExecEnv) != "1" {
		b = &inprocBackend{exec: b, clock: time.Now}
	}
	return &door{backend: b, now: func() float64 { return time.Since(processStart).Seconds() }}
}

// execBackend runs `<python> <script> <args...>` like backend.run_tool.
type execBackend struct {
	python func() (string, error)
}

// pythonInterpreter is $NSEARCH_PYTHON, else python3, else python, from PATH.
func pythonInterpreter() (string, error) {
	if p := os.Getenv("NSEARCH_PYTHON"); p != "" {
		return p, nil
	}
	for _, name := range []string{"python3", "python"} {
		if p, err := exec.LookPath(name); err == nil {
			return p, nil
		}
	}
	return "", errors.New("no python interpreter on PATH (set NSEARCH_PYTHON)")
}

func (b execBackend) Run(argv []string, timeout float64) Raw {
	exe := pyPath(argv[0])
	if _, err := os.Stat(argv[0]); err != nil {
		return Raw{State: Unavailable, Reason: "unavailable: tool not found at " + exe}
	}
	python, err := b.python()
	if err != nil {
		return Raw{State: Unavailable, Reason: "unavailable: " + err.Error()}
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout*float64(time.Second)))
	defer cancel()
	cmd := exec.CommandContext(ctx, python, argv...)
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	cmd.WaitDelay = 100 * time.Millisecond // a grandchild holding a pipe must not outlive the deadline
	t0 := time.Now()
	runErr := cmd.Run()
	if ctx.Err() != nil {
		return Raw{State: Unavailable, Reason: "unavailable: timed out after " + formatG3(timeout) + "s",
			MS: int(timeout * 1000)}
	}
	ms := int(time.Since(t0) / time.Millisecond)
	rc := 0
	var exitErr *exec.ExitError
	switch {
	case errors.As(runErr, &exitErr):
		rc = exitCode(exitErr)
	case runErr != nil:
		return Raw{State: Unavailable, Reason: "unavailable: " + runErr.Error()}
	}
	return Classify(rc, textOf(out.Bytes()), textOf(errb.Bytes()), ms)
}

// exitCode is Popen.returncode: the exit status, or -N for a signal N.
func exitCode(e *exec.ExitError) int {
	if ws, ok := e.Sys().(syscall.WaitStatus); ok && ws.Signaled() {
		return -int(ws.Signal())
	}
	return e.ExitCode()
}
