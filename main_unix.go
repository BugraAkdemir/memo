//go:build unix

package main

import "syscall"

// detachAttr starts the spawned backend in its own session (setsid) so it
// survives this process exiting and isn't killed by a signal delivered to
// this terminal's foreground process group (e.g. Ctrl+C at the shell).
func detachAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{Setsid: true}
}
