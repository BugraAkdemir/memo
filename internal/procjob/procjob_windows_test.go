//go:build windows

package procjob

import (
	"os/exec"
	"testing"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

// A bound process must die when the job's last handle closes — the same thing
// the OS does when the backend exits, however it exits. Here the test closes
// the handle itself, so it uses its own job rather than the process-wide one.
func TestBoundProcessDiesWhenTheJobCloses(t *testing.T) {
	cmd := exec.Command("ping", "-n", "60", "127.0.0.1")
	if err := cmd.Start(); err != nil {
		t.Skipf("cannot start a child: %v", err)
	}
	defer func() { _ = cmd.Process.Kill() }()

	h, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	info := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{BasicLimitInformation: windows.JOBOBJECT_BASIC_LIMIT_INFORMATION{LimitFlags: windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE}}
	if _, err := windows.SetInformationJobObject(h, windows.JobObjectExtendedLimitInformation, uintptr(unsafe.Pointer(&info)), uint32(unsafe.Sizeof(info))); err != nil {
		t.Fatal(err)
	}
	ph, err := windows.OpenProcess(windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE, false, uint32(cmd.Process.Pid))
	if err != nil {
		t.Fatal(err)
	}
	if err := windows.AssignProcessToJobObject(h, ph); err != nil {
		t.Fatal(err)
	}
	_ = windows.CloseHandle(ph)

	done := make(chan struct{})
	go func() { _ = cmd.Wait(); close(done) }()
	_ = windows.CloseHandle(h)

	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("the bound process outlived its job")
	}
}

func TestBindPutsAnEngineIntoTheBackendJob(t *testing.T) {
	cmd := exec.Command("ping", "-n", "60", "127.0.0.1")
	if err := cmd.Start(); err != nil {
		t.Skipf("cannot start a child: %v", err)
	}
	defer func() { _ = cmd.Process.Kill(); _ = cmd.Wait() }()

	if err := bind(cmd.Process); err != nil {
		t.Fatalf("bind: %v", err)
	}
	if job == 0 {
		t.Fatal("bind succeeded without creating the backend's job")
	}
	if err := bind(nil); err == nil {
		t.Error("bind(nil) must fail, not panic or pretend")
	}
}
