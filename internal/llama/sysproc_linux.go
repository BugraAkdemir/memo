//go:build linux

package llama

import "syscall"

// newSysProcAttr puts the child in its own process group so forceKill/killByPort
// can kill the entire tree without affecting the parent. Pdeathsig is NOT used
// because it's incompatible with Setpgid in Go (runtime thread reuse triggers
// premature child death). Process-group kill handles cleanup instead.
func newSysProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{
		Setpgid: true,
	}
}
