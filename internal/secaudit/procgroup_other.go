//go:build !unix

package secaudit

import "os/exec"

// killTreeOnCancel keeps the default kill of the direct child: keytool.exe runs the JVM in-process.
func killTreeOnCancel(cmd *exec.Cmd) {}
