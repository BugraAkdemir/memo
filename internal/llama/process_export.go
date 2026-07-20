package llama

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
	"time"

	"memo/internal/logx"
)

// KillByPort finds the process listening on the given TCP port (if any)
// and terminates it — SIGTERM first, SIGKILL after a 3s grace period.
// Server.killByPort delegates here for llama-server's own orphan cleanup;
// exposed at package level so other packages in this module managing their
// own subprocess on a known port (e.g. internal/swarm's rpc-server worker)
// can reuse the same behavior instead of reimplementing the retry/kill
// logic themselves.
func KillByPort(port int) error {
	pid := pidListeningOnPort(port)
	if pid <= 0 {
		logx.Printf("llama: nothing found on port %d (lsof/fuser/netstat unavailable or port free)", port)
		return nil
	}

	proc, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("find process %d: %w", pid, err)
	}

	logx.Printf("llama: killing external PID %d on port %d", pid, port)
	processSignalTerm(proc)

	for i := 0; i < 6; i++ {
		time.Sleep(500 * time.Millisecond)
		if !processIsAlive(proc) {
			logx.Printf("llama: process %d exited cleanly", pid)
			return nil
		}
	}
	logx.Printf("llama: force-killing PID %d", pid)
	proc.Kill()
	return nil
}

// NewSysProcAttr exposes this package's process-group isolation setup
// (Setpgid, deliberately no Pdeathsig — see the platform-specific
// sysproc_*.go files' doc comments for why) for reuse by other packages in
// this module that manage their own subprocess and want the same
// kill-the-whole-tree behavior ForceKillCmd/KillByPort rely on.
func NewSysProcAttr() *syscall.SysProcAttr { return newSysProcAttr() }

// ForceKillCmd exposes forceKillCmd for reuse — SIGKILLs cmd's whole
// process group (falling back to just the process itself if the group
// lookup fails), then waits up to 3s on waitDone before giving up (pass
// nil to skip waiting).
func ForceKillCmd(cmd *exec.Cmd, waitDone chan struct{}) { forceKillCmd(cmd, waitDone) }

// ProcessSignalTerm exposes processSignalTerm for reuse — sends the
// platform-appropriate graceful-shutdown signal (SIGTERM on Unix; see
// process_windows.go for the Windows equivalent).
func ProcessSignalTerm(p *os.Process) error { return processSignalTerm(p) }
