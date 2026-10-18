//go:build unix

package helpbuild

import (
	"os/exec"
	"syscall"
)

// killGroupOnCancel starts a child (pdftotext) in its own process group and
// makes a cancel (the timeout) kill the whole group, so a wrapper script
// that forks the real program leaves nothing running. Python's
// subprocess.run kills only the direct child.
func killGroupOnCancel(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error { return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL) }
}
