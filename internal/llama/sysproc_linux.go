//go:build linux

package llama

import "syscall"

// newSysProcAttr puts the child in its own process group so we can kill the
// entire tree cleanly. Note: Pdeathsig cannot be used together with Setpgid
// in Go — the Go runtime recycles OS threads, which triggers Pdeathsig on the
// child prematurely. Process-group kill (killByPort / forceKill) handles
// cleanup instead.
func newSysProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{
		Pdeathsig: syscall.SIGKILL,
	}
}
