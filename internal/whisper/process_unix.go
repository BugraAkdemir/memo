//go:build !windows

package whisper

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
		logx.Printf("whisper: lsof for port %d failed: %v", port, err)
	}

	out, err = exec.Command("fuser", fmt.Sprintf("%d/tcp", port)).Output()
	if err == nil {
		parts := strings.SplitN(string(out), ":", 2)
		if len(parts) == 2 {
			for _, tok := range strings.Fields(parts[1]) {
				pid := 0
				if n, _ := fmt.Sscanf(tok, "%d", &pid); n == 1 && pid > 0 {
					return pid
				}
			}
		}
	} else {
		logx.Printf("whisper: fuser for port %d failed: %v", port, err)
	}

	return 0
}
