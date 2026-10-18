//go:build !unix

package helpbuild

import "os/exec"

// killGroupOnCancel keeps the default kill of the direct child, which is
// what Python's subprocess.run does on a timeout.
func killGroupOnCancel(cmd *exec.Cmd) {}
