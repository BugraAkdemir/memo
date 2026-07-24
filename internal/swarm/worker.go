// SPDX-License-Identifier: AGPL-3.0-or-later

// Package swarm implements Memo Swarm — pooling multiple machines' compute
// via llama.cpp's RPC backend to run one model too large for any single
// one of them. See PLAN_memo_swarm.md for the full design.
package swarm

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"sync"
	"time"

	"memo/internal/config"
	"memo/internal/llama"
	"memo/internal/logx"
)

// RPCWorker manages a local rpc-server subprocess — the "Join" role in
// Memo Swarm. Deliberately not built on llama.Server: rpc-server is a
// structurally different process (no model file, no ctx-size/gpu-layers,
// no HTTP health endpoint), so it doesn't fit Server's already-large Start
// signature. Reuses llama package's exported low-level process-management
// primitives (NewSysProcAttr, KillByPort, ForceKillCmd, ProcessSignalTerm)
// instead of duplicating that platform-specific logic.
type RPCWorker struct {
	mu       sync.RWMutex
	cmd      *exec.Cmd
	port     int
	waitDone chan struct{}
	logFile  *os.File
}

// NewRPCWorker creates a new rpc-server manager bound to the given port.
func NewRPCWorker(port int) *RPCWorker {
	return &RPCWorker{port: port}
}

// Start launches rpc-server on w's configured port, binding 0.0.0.0 so a
// coordinator on the LAN (or reachable via Tailscale) can dial in.
// threads<=0 leaves rpc-server's own default thread count in place.
func (w *RPCWorker) Start(binaryPath string, threads int) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.cmd != nil && w.cmd.Process != nil {
		return fmt.Errorf("swarm: rpc-server already running (PID %d)", w.cmd.Process.Pid)
	}
	if binaryPath == "" {
		return fmt.Errorf("swarm: rpc-server binary path required")
	}
	if _, err := os.Stat(binaryPath); err != nil {
		return fmt.Errorf("swarm: rpc-server binary not found at %s: %w", binaryPath, err)
	}

	// Clear a possible orphaned prior instance before binding — mirrors
	// llama.Server.Start's own unconditional pre-flight port clear: no
	// Pdeathsig is set here either (see llama.NewSysProcAttr's doc comment,
	// same reasoning — incompatible with Setpgid), so a crashed parent Memo
	// process can orphan this one too.
	if err := llama.KillByPort(w.port); err != nil {
		logx.Printf("swarm: could not clear port %d before starting rpc-server: %v", w.port, err)
	}

	args := []string{"--host", "0.0.0.0", "--port", fmt.Sprintf("%d", w.port)}
	if threads > 0 {
		args = append(args, "--threads", fmt.Sprintf("%d", threads))
	}
	logx.Printf("swarm: launching %s %v", binaryPath, args)

	w.cmd = exec.Command(binaryPath, args...)

	if w.logFile != nil {
		w.logFile.Close()
		w.logFile = nil
	}
	logPath := config.DataPath(fmt.Sprintf("rpc-server-%d.log", w.port))
	if f, ferr := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644); ferr == nil {
		w.logFile = f
		w.cmd.Stdout = f
		w.cmd.Stderr = f
	} else {
		logx.Printf("swarm: could not open %s (%v), falling back to inherited stdout/stderr", logPath, ferr)
		w.cmd.Stdout = os.Stdout
		w.cmd.Stderr = os.Stderr
	}

	w.cmd.SysProcAttr = llama.NewSysProcAttr()
	w.waitDone = make(chan struct{})

	if err := w.cmd.Start(); err != nil {
		w.cmd = nil
		return fmt.Errorf("swarm: rpc-server start failed: %w", err)
	}

	logx.Printf("swarm: rpc-server started (PID %d, port %d)", w.cmd.Process.Pid, w.port)
	logx.GoRecover("swarm.RPCWorker.monitor", w.monitor)
	return nil
}

// monitor waits for the process to exit (expectedly via Stop, or
// unexpectedly — a crash) and clears cmd so IsRunning/Start reflect it.
func (w *RPCWorker) monitor() {
	w.mu.RLock()
	cmd := w.cmd
	done := w.waitDone
	w.mu.RUnlock()
	if cmd == nil {
		return
	}

	err := cmd.Wait()

	w.mu.Lock()
	if done != nil {
		close(done)
	}
	// Only clear state if this monitor goroutine still owns the current
	// cmd — a Start() call that raced past a Stop() could otherwise have
	// this stale goroutine clobber a newer, still-running process's state.
	if w.cmd == cmd {
		if err != nil {
			logx.Printf("swarm: rpc-server exited: %v", err)
		} else {
			logx.Printf("swarm: rpc-server exited cleanly")
		}
		w.cmd = nil
	}
	w.mu.Unlock()
}

// Stop gracefully shuts down rpc-server — SIGTERM, then up to 5s before
// force-killing the whole process group.
func (w *RPCWorker) Stop() error {
	w.mu.Lock()
	if w.cmd == nil || w.cmd.Process == nil {
		w.mu.Unlock()
		return nil
	}
	cmd := w.cmd
	waitDone := w.waitDone
	w.mu.Unlock()

	llama.ProcessSignalTerm(cmd.Process)

	select {
	case <-waitDone:
		logx.Printf("swarm: rpc-server stopped gracefully")
	case <-time.After(5 * time.Second):
		logx.Printf("swarm: graceful shutdown timed out, force killing")
		llama.ForceKillCmd(cmd, waitDone)
	}

	w.mu.Lock()
	w.cmd = nil
	if w.logFile != nil {
		w.logFile.Close()
		w.logFile = nil
	}
	w.mu.Unlock()
	return nil
}

// IsRunning reports whether rpc-server is currently running.
func (w *RPCWorker) IsRunning() bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.cmd != nil && w.cmd.Process != nil
}

// WaitReady polls the configured port until a raw TCP connection succeeds
// or timeout elapses. Unlike llama.Server.WaitReady (which GETs
// /v1/models), rpc-server has no HTTP health endpoint at all, so bare port
// reachability is the closest available readiness signal.
func (w *RPCWorker) WaitReady(timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	w.mu.RLock()
	port := w.port
	w.mu.RUnlock()
	addr := fmt.Sprintf("127.0.0.1:%d", port)

	for time.Now().Before(deadline) {
		if !w.IsRunning() {
			return fmt.Errorf("swarm: rpc-server process exited during startup")
		}
		conn, err := net.DialTimeout("tcp", addr, 500*time.Millisecond)
		if err == nil {
			conn.Close()
			logx.Printf("swarm: rpc-server ready on port %d", port)
			return nil
		}
		time.Sleep(200 * time.Millisecond)
	}
	return fmt.Errorf("swarm: rpc-server failed to become ready within %v", timeout)
}
