// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build windows

package updater

import (
	"fmt"
	"syscall"
	"unsafe"
)

var shellExecute = syscall.NewLazyDLL("shell32.dll").NewProc("ShellExecuteW")

// RunInstaller starts a downloaded Setup.exe the way Explorer would, so its
// "run as administrator" manifest shows the normal UAC prompt (CreateProcess
// would refuse it with ERROR_ELEVATION_REQUIRED). The caller exits afterwards;
// Inno Setup's CloseApplications handles anything still running.
func RunInstaller(path string) error {
	verb, _ := syscall.UTF16PtrFromString("open")
	file, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return err
	}
	// SW_SHOWNORMAL = 1. ShellExecute returns a value > 32 on success.
	r, _, _ := shellExecute.Call(0, uintptr(unsafe.Pointer(verb)), uintptr(unsafe.Pointer(file)), 0, 0, 1)
	if r <= 32 {
		return fmt.Errorf("updater: could not start the installer (ShellExecute code %d)", r)
	}
	return nil
}
