//go:build !windows

package llama

import (
	"fmt"
	"memo/internal/logx"
	"os"
	"os/exec"
	"strings"
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
			logx.Printf("llama: WARNING — process may not have exited cleanly")
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

func pidListeningOnPort(port int) int {
	out, err := exec.Command("lsof", "-ti", fmt.Sprintf("tcp:%d", port)).Output()
	if err == nil {
		for _, line := range strings.Fields(string(out)) {
			pid := 0
			if n, _ := fmt.Sscanf(line, "%d", &pid); n == 1 && pid > 0 {
				return pid
			}
		}
	} else {
		logx.Printf("llama: lsof for port %d failed: %v", port, err)
	}

	// fuser writes the "PORT/tcp:" label to stderr and only the bare,
	// space-separated PID list to stdout — Output() captures stdout only,
	// so there is never a ":" in out to split on.
	out, err = exec.Command("fuser", fmt.Sprintf("%d/tcp", port)).Output()
	if err == nil {
		for _, tok := range strings.Fields(string(out)) {
			pid := 0
			if n, _ := fmt.Sscanf(tok, "%d", &pid); n == 1 && pid > 0 {
				return pid
			}
		}
	} else {
		logx.Printf("llama: fuser for port %d failed: %v", port, err)
	}

	return 0
}
