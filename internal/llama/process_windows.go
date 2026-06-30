//go:build windows

package llama

import (
	"fmt"
	"memo/internal/logx"
	"os"
	"os/exec"
	"strings"
	"time"
)

func processSignalTerm(p *os.Process) error {
	// Windows has no SIGTERM; Kill is the only reliable signal.
	return p.Kill()
}

func processIsAlive(p *os.Process) bool {
	out, err := exec.Command("tasklist", "/FI", fmt.Sprintf("PID eq %d", p.Pid), "/NH", "/FO", "CSV").Output()
	if err != nil {
		return false
	}
	return strings.Contains(string(out), fmt.Sprintf("%d", p.Pid))
}

func forceKillCmd(cmd *exec.Cmd, waitDone chan struct{}) {
	_ = cmd.Process.Kill()

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
	out, err := exec.Command("netstat", "-ano").Output()
	if err != nil {
		logx.Printf("llama: netstat failed: %v", err)
		return 0
	}
	target := fmt.Sprintf(":%d", port)
	for _, line := range strings.Split(string(out), "\n") {
		if !strings.Contains(line, target) {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 5 {
			continue
		}
		pid := 0
		if n, _ := fmt.Sscanf(fields[len(fields)-1], "%d", &pid); n == 1 && pid > 0 {
			return pid
		}
	}
	return 0
}
