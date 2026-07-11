//go:build !windows

package llama

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"
)

// TestHelperProcess_ListenOnPort is not a real test — it's a re-exec target
// (classic os/exec-testing pattern) that TestKillByPort_KillsOrphanProcess
// spawns as a separate process to stand in for an orphaned llama-server:
// binds an ephemeral port, prints it, and blocks, so the parent test has a
// real OS-level listener with its own PID to kill.
func TestHelperProcess_ListenOnPort(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		fmt.Println("LISTEN_FAILED")
		os.Exit(1)
	}
	defer l.Close()
	fmt.Println(l.Addr().(*net.TCPAddr).Port)
	time.Sleep(30 * time.Second)
}

// TestKillByPort_KillsOrphanProcess is a regression test for the
// stuck-embedding-port bug: a process this Server never spawned (simulating
// a llama-server orphaned by a previous, abnormally-terminated Memo run —
// see the Pdeathsig note in Start()) is listening on a port. killByPort must
// find and kill it via lsof/fuser-based discovery, not just PID tracking.
func TestKillByPort_KillsOrphanProcess(t *testing.T) {
	if _, err := exec.LookPath("lsof"); err != nil {
		if _, err := exec.LookPath("fuser"); err != nil {
			t.Skip("neither lsof nor fuser available in this environment")
		}
	}

	cmd := exec.Command(os.Args[0], "-test.run=^TestHelperProcess_ListenOnPort$")
	cmd.Env = append(os.Environ(), "GO_WANT_HELPER_PROCESS=1")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatalf("StdoutPipe() error = %v", err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatalf("failed to start helper process: %v", err)
	}
	t.Cleanup(func() { _ = cmd.Process.Kill() })

	line, err := bufio.NewReader(stdout).ReadString('\n')
	if err != nil {
		t.Fatalf("failed to read helper's port: %v", err)
	}
	line = strings.TrimSpace(line)
	if line == "LISTEN_FAILED" {
		t.Fatal("helper process failed to bind a port")
	}
	port, err := strconv.Atoi(line)
	if err != nil {
		t.Fatalf("unexpected helper output %q: %v", line, err)
	}

	// Give the OS a moment to make the listener visible to lsof/fuser.
	time.Sleep(200 * time.Millisecond)

	s := &Server{}
	if err := s.killByPort(port); err != nil {
		t.Fatalf("killByPort(%d) error = %v", port, err)
	}

	// This test is the helper's direct parent, so a killed-but-unreaped
	// child sits as a zombie — processIsAlive's signal(0) check would still
	// report it "alive". cmd.Wait() is the correct liveness proof here: it
	// only returns once the process has actually exited.
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case <-done:
	case <-time.After(4 * time.Second):
		t.Fatalf("orphan process (PID %d) still alive after killByPort", cmd.Process.Pid)
	}
}
