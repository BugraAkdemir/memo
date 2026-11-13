//go:build !windows

// SPDX-License-Identifier: AGPL-3.0-or-later

package procjob

import (
	"os"
	"os/exec"
	"syscall"
)

type managedTreeHandle struct{}

// Bind does nothing outside Windows (see the package doc).
func Bind(*os.Process, string) {}

// EnsureSelf is a no-op on Unix; process lifetime is governed by process groups
// and the owner control pipe rather than Windows Job Objects.
func EnsureSelf() error { return nil }

// ManagedAttr places owned backend subprocesses into their own process group so
// hung teardown kills the backend plus its local engines without touching
// DetachedAttr siblings or the desktop owner.
func ManagedAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{Setpgid: true}
}

// UIAttr places the desktop UI subprocess into its own process group.
func UIAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{Setpgid: true}
}

// DetachedAttr places external applications (open_app, browseropen,
// memo-updater) into a new session so they survive desktop session exit.
func DetachedAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{Setsid: true}
}

func attachManagedTree(*os.Process) *managedTreeHandle { return nil }

func killManagedTree(cmd *exec.Cmd, _ *managedTreeHandle) {
	KillProcessTree(cmd)
}

func releaseManagedTree(*managedTreeHandle) {}

// KillProcessTree force-kills the child process's entire process group on Unix.
func KillProcessTree(cmd *exec.Cmd) {
	if cmd == nil || cmd.Process == nil || cmd.Process.Pid <= 0 {
		return
	}
	if err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL); err != nil {
		_ = cmd.Process.Kill()
	}
}

func terminateGraceful(cmd *exec.Cmd) {
	if cmd == nil || cmd.Process == nil || cmd.Process.Pid <= 0 {
		return
	}
	if err := syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM); err != nil {
		_ = cmd.Process.Signal(syscall.SIGTERM)
	}
}

func processAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	err := syscall.Kill(pid, 0)
	return err == nil || err == syscall.EPERM
}
