//go:build !windows

package whisper

import (
	"fmt"
	"memo/internal/logx"
	"os"
	"os/exec"
	"syscall"
	"time"
)

func processSignalTerm(p *os.Process) error {
	return p.Signal(syscall.SIGTERM)
}

func processIsAlive(p *os.Process) bool {
	return p.Signal(syscall.Signal(0)) == nil
}

func forceKillCmd(cmd *exec.Cmd, waitDone chan struct{}) {
	pid := cmd.Process.Pid
	pgid, err := syscall.Getpgid(pid)
	if err == nil && pgid != 0 {
		_ = syscall.Kill(-pgid, syscall.SIGKILL)
	} else {
		_ = cmd.Process.Kill()
	}

	if waitDone != nil {
		select {
		case <-waitDone:
		case <-time.After(3 * time.Second):
			logx.Printf("whisper: WARNING — process may not have exited cleanly")
		}
	}
}

func killPID(pid int) error {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("find %d: %w", pid, err)
	}
	return processSignalTerm(proc)
}
