//go:build !windows

package ngrok

import (
	"os/exec"
	"syscall"
	"time"
)

// newSysProcAttr puts the child in its own process group so killProcessTree
// can kill the entire tree without taking down the parent app (see llama's
// sysproc_linux.go for the same pattern).
func newSysProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{
		Setpgid: true,
	}
}

// killProcessTree terminates the ngrok process and its children. The process
// group kill is only safe because Start set Setpgid, giving ngrok its own
// group — otherwise -pgid would be the app's own group.
func killProcessTree(cmd *exec.Cmd) {
	pgid, err := syscall.Getpgid(cmd.Process.Pid)
	if err == nil && pgid != cmd.Process.Pid {
		// Not a group leader (Setpgid somehow not applied) — kill only the process.
		cmd.Process.Kill()
		return
	}
	if err == nil {
		syscall.Kill(-pgid, syscall.SIGTERM)
		time.Sleep(200 * time.Millisecond)
		syscall.Kill(-pgid, syscall.SIGKILL)
	} else {
		cmd.Process.Kill()
	}
}
