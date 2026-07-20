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

	// fuser writes the "PORT/tcp:" label to stderr and only the bare,
	// space-separated PID list to stdout — Output() captures stdout only, so
	// there is never a ":" in out to split on. This copy used to
	// SplitN(out, ":", 2) and require len(parts) == 2, which could therefore
	// never be true: fuser would succeed, the branch would be skipped, and
	// this function returned 0 as if the port were free. The identical bug
	// was already found and fixed in llama's copy of this file; whisper's
	// was left behind, so whisper's port cleanup has never once worked on a
	// machine without lsof installed.
	out, err = exec.Command("fuser", fmt.Sprintf("%d/tcp", port)).Output()
	if err == nil {
		for _, tok := range strings.Fields(string(out)) {
			pid := 0
			if n, _ := fmt.Sscanf(tok, "%d", &pid); n == 1 && pid > 0 {
				return pid
			}
		}
	} else {
		logx.Printf("whisper: fuser for port %d failed: %v", port, err)
	}

	return 0
}
