//go:build windows

package main

import "syscall"

// detachAttr starts the spawned backend in its own process group so it
// survives this process exiting and doesn't receive a Ctrl+C/Ctrl+Break
// meant for this console.
func detachAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP}
}
