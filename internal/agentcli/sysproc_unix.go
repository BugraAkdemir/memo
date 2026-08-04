// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build !windows

package agentcli

import (
	"os/exec"
	"syscall"
)

// newSysProcAttr puts the claude/codex subprocess in its own process group,
// same pattern as internal/llama's sysproc_linux.go/sysproc_darwin.go — so
// killProcessGroup below can kill the whole tree it may have spawned
// (--dangerously-skip-permissions/--dangerously-bypass-approvals-and-sandbox
// let it run shell commands, git, an editor, ... as real children) instead
// of only the direct claude/codex process.
func newSysProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{Setpgid: true}
}

// killProcessGroup is cmd.Cancel — invoked by exec.CommandContext instead of
// its default cmd.Process.Kill(), which only ever killed the direct child.
// A grandchild that inherited the stdout pipe and outlives that single-PID
// kill is what BUG_REPORT.md's LK-1 was really about: cliProcessWaitDelay
// (claude_code.go) unblocks the Go-side scanner reading that pipe, but
// doesn't stop whatever the orphaned process is still doing with its
// filesystem/command authority. Killing the whole group closes both gaps at
// once.
func killProcessGroup(cmd *exec.Cmd) error {
	pgid, err := syscall.Getpgid(cmd.Process.Pid)
	if err != nil || pgid == 0 {
		return cmd.Process.Kill()
	}
	return syscall.Kill(-pgid, syscall.SIGKILL)
}
