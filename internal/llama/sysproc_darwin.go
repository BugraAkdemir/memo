//go:build darwin

package llama

import "syscall"

// macOS has no Pdeathsig, so we can safely use Setpgid to put the child in
// its own process group. forceKillCmd then sends SIGKILL to -pgid (the child's
// group only), without affecting the parent Memo process.
func newSysProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{
		Setpgid: true,
	}
}
