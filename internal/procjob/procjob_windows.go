//go:build windows

// SPDX-License-Identifier: AGPL-3.0-or-later

package procjob

import (
	"fmt"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"

	"memo/internal/logx"
)

var (
	jobOnce sync.Once
	job     windows.Handle
	jobErr  error

	selfOnce sync.Once
	selfErr  error
)

// theJob creates Memo's kill-on-close + breakaway job once. The handle is kept
// for the life of the process so OS close guarantees child termination even on
// abrupt termination, while JOB_OBJECT_LIMIT_BREAKAWAY_OK lets DetachedAttr
// break external apps and the updater out cleanly.
func theJob() (windows.Handle, error) {
	jobOnce.Do(func() {
		h, err := newKillOnCloseJob()
		if err != nil {
			jobErr = err
			return
		}
		job = h
	})
	return job, jobErr
}

func newKillOnCloseJob() (windows.Handle, error) {
	h, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return 0, fmt.Errorf("CreateJobObject: %w", err)
	}
	info := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{
		BasicLimitInformation: windows.JOBOBJECT_BASIC_LIMIT_INFORMATION{
			LimitFlags: windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE | windows.JOB_OBJECT_LIMIT_BREAKAWAY_OK,
		},
	}
	if _, err := windows.SetInformationJobObject(h, windows.JobObjectExtendedLimitInformation,
		uintptr(unsafe.Pointer(&info)), uint32(unsafe.Sizeof(info))); err != nil {
		_ = windows.CloseHandle(h)
		return 0, fmt.Errorf("SetInformationJobObject: %w", err)
	}
	return h, nil
}

// EnsureSelf assigns the current process to Memo's Job Object so all managed
// children (desktop UI, owned backend, engines) inherit job membership from
// CreateProcess onward with zero start-to-assignment race. Safe to call repeatedly.
func EnsureSelf() error {
	selfOnce.Do(func() {
		j, err := theJob()
		if err != nil {
			selfErr = err
			return
		}
		if err := windows.AssignProcessToJobObject(j, windows.CurrentProcess()); err != nil {
			selfErr = fmt.Errorf("AssignProcessToJobObject(self): %w", err)
		}
	})
	return selfErr
}

func canBreakaway() bool {
	var info windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION
	if err := windows.QueryInformationJobObject(
		0,
		windows.JobObjectExtendedLimitInformation,
		uintptr(unsafe.Pointer(&info)),
		uint32(unsafe.Sizeof(info)),
		nil,
	); err != nil {
		return false
	}
	return info.BasicLimitInformation.LimitFlags&windows.JOB_OBJECT_LIMIT_BREAKAWAY_OK != 0
}

// ManagedAttr configures child processes managed directly under the Memo
// session (e.g. owned headless backend), hiding console windows on Windows
// while keeping Job Object inheritance intact.
func ManagedAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: windows.CREATE_NO_WINDOW | windows.CREATE_NEW_PROCESS_GROUP,
	}
}

// UIAttr configures the desktop Flutter UI child so its GUI window renders
// normally while inheriting Job Object containment from the owner.
func UIAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{
		CreationFlags: windows.CREATE_NEW_PROCESS_GROUP,
	}
}

// DetachedAttr configures external applications (open_app, browseropen,
// memo-updater) that must survive Memo exiting or being terminated.
func DetachedAttr() *syscall.SysProcAttr {
	flags := uint32(windows.CREATE_NEW_PROCESS_GROUP)
	if canBreakaway() {
		flags |= windows.CREATE_BREAKAWAY_FROM_JOB
	}
	return &syscall.SysProcAttr{
		CreationFlags: flags,
	}
}

type managedTreeHandle struct {
	job windows.Handle
}

func attachManagedTree(p *os.Process) *managedTreeHandle {
	if p == nil {
		return nil
	}
	j, err := newKillOnCloseJob()
	if err != nil {
		return nil
	}
	h, err := windows.OpenProcess(windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE, false, uint32(p.Pid))
	if err != nil {
		_ = windows.CloseHandle(j)
		return nil
	}
	defer windows.CloseHandle(h)
	if err := windows.AssignProcessToJobObject(j, h); err != nil {
		_ = windows.CloseHandle(j)
		return nil
	}
	return &managedTreeHandle{job: j}
}

func killManagedTree(cmd *exec.Cmd, tree *managedTreeHandle) {
	if tree != nil && tree.job != 0 {
		_ = windows.TerminateJobObject(tree.job, 1)
		_ = windows.CloseHandle(tree.job)
		tree.job = 0
	}
	KillProcessTree(cmd)
}

func releaseManagedTree(tree *managedTreeHandle) {
	if tree != nil && tree.job != 0 {
		_ = windows.CloseHandle(tree.job)
		tree.job = 0
	}
}

// KillProcessTree terminates a child process on Windows. When coupled with an
// attached Job Object (EnsureSelf / attachManagedTree), children inside the job
// terminate together while DetachedAttr siblings remain untouched.
func KillProcessTree(cmd *exec.Cmd) {
	if cmd == nil || cmd.Process == nil {
		return
	}
	_ = cmd.Process.Kill()
}

func terminateGraceful(cmd *exec.Cmd) {
	KillProcessTree(cmd)
}

func processAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
	if err != nil {
		return false
	}
	defer windows.CloseHandle(h)
	var code uint32
	if err := windows.GetExitCodeProcess(h, &code); err != nil {
		return false
	}
	return code == 259 // STILL_ACTIVE
}

// Bind puts a started engine process into the backend's job. Best-effort: a
// failure is logged and the engine keeps running as before.
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
