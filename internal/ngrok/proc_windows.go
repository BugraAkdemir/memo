//go:build windows

package ngrok

import (
	"os/exec"
	"strconv"
	"syscall"
)

func newSysProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{}
}

// killProcessTree terminates the ngrok process and all its children.
// Windows has no process groups, so we use taskkill /T to kill the whole tree.
func killProcessTree(cmd *exec.Cmd) {
	if cmd.Process != nil {
		pid := strconv.Itoa(cmd.Process.Pid)
		exec.Command("taskkill", "/F", "/T", "/PID", pid).Run() //nolint:errcheck
	}
}
