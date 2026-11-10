//go:build windows

// SPDX-License-Identifier: AGPL-3.0-or-later

package procjob

import (
	"fmt"
	"os"
	"sync"
	"unsafe"

	"golang.org/x/sys/windows"

	"memo/internal/logx"
)

var (
	jobOnce sync.Once
	job     windows.Handle
	jobErr  error
)

// theJob creates the backend's kill-on-close job once. The handle is kept for
// the life of the process on purpose (see the package doc).
func theJob() (windows.Handle, error) {
	jobOnce.Do(func() {
		h, err := windows.CreateJobObject(nil, nil)
		if err != nil {
			jobErr = fmt.Errorf("CreateJobObject: %w", err)
			return
		}
		info := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{
			BasicLimitInformation: windows.JOBOBJECT_BASIC_LIMIT_INFORMATION{
				LimitFlags: windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE,
			},
		}
		if _, err := windows.SetInformationJobObject(h, windows.JobObjectExtendedLimitInformation,
			uintptr(unsafe.Pointer(&info)), uint32(unsafe.Sizeof(info))); err != nil {
			_ = windows.CloseHandle(h)
			jobErr = fmt.Errorf("SetInformationJobObject: %w", err)
			return
		}
		job = h
	})
	return job, jobErr
}

// Bind puts a started engine process into the backend's job. Best-effort: a
// failure is logged and the engine keeps running as before (the launcher and
// `memo --kill` still stop it by name).
func Bind(p *os.Process, name string) {
	if err := bind(p); err != nil {
		logx.Printf("procjob: %s (PID %d) not bound to the backend's lifetime: %v", name, pidOf(p), err)
	}
}

func bind(p *os.Process) error {
	if p == nil {
		return fmt.Errorf("no process")
	}
	j, err := theJob()
	if err != nil {
		return err
	}
	h, err := windows.OpenProcess(windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE, false, uint32(p.Pid))
	if err != nil {
		return fmt.Errorf("OpenProcess: %w", err)
	}
	defer windows.CloseHandle(h)
	if err := windows.AssignProcessToJobObject(j, h); err != nil {
		return fmt.Errorf("AssignProcessToJobObject: %w", err)
	}
	return nil
}

func pidOf(p *os.Process) int {
	if p == nil {
		return 0
	}
	return p.Pid
}
