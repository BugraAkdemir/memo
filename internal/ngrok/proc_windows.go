//go:build windows

package ngrok

import (
	"os/exec"
	"syscall"
)

func newSysProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{}
}

func killProcessTree(cmd *exec.Cmd) {
	cmd.Process.Kill()
}
