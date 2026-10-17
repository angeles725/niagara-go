//go:build unix

package secaudit

import (
	"os/exec"
	"syscall"
)

// killTreeOnCancel starts the child in its own process group and makes a timeout kill the whole
// group, so a wrapper script that forks java instead of exec'ing it leaves nothing running.
func killTreeOnCancel(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error { return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL) }
}
