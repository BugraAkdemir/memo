//go:build !windows

package whisper

import (
	"net"
	"os"
	"os/exec"
	"testing"
)

// hasPortDiscoveryTool reports whether either external tool
// pidListeningOnPort depends on is actually installed. Neither is
// guaranteed: lsof is absent on plenty of minimal Linux installs (it was
// missing on the machine this bug was reported from), and fuser ships in
// psmisc, which is also optional.
func hasPortDiscoveryTool() bool {
	for _, bin := range []string{"lsof", "fuser"} {
		if _, err := exec.LookPath(bin); err == nil {
			return true
		}
	}
	return false
}

// TestPidListeningOnPort_FindsARealListener is the regression test for a bug
// that made whisper's port cleanup a permanent no-op on any machine without
// lsof: the fuser branch split fuser's stdout on ":" and required two parts,
// but fuser writes the "PORT/tcp:" label to stderr and only the bare,
// space-separated PID list to stdout. There was never a ":" to split on, so
// the branch could not execute, and this function reported 0 — "port is
// free" — for a port that was very much in use. Everything downstream
// (Stop's by-port path, and Start's new pre-flight) silently did nothing.
//
// Deliberately asserts against this test process's own PID rather than
// spawning and killing a victim: the thing worth locking down is that a
// listening socket is attributed to the right process at all.
func TestPidListeningOnPort_FindsARealListener(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()
	port := ln.Addr().(*net.TCPAddr).Port

	got := pidListeningOnPort(port)
	if got == 0 && !hasPortDiscoveryTool() {
		t.Skip("neither lsof nor fuser is installed — port discovery cannot work here")
	}
	if got != os.Getpid() {
		t.Errorf("pidListeningOnPort(%d) = %d, want this process (%d)", port, got, os.Getpid())
	}
}

// A free port must report 0, so killByPort stays a no-op instead of
// SIGTERMing some unrelated process.
func TestPidListeningOnPort_FreePortReportsNobody(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	ln.Close() // release it before asking

	if got := pidListeningOnPort(port); got != 0 {
		t.Errorf("pidListeningOnPort(%d) on a closed port = %d, want 0", port, got)
	}
}

// killByPort must not error (and must not kill anything) when the port is
// already free — Start calls it unconditionally on every launch.
func TestKillByPort_NoOpOnFreePort(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	ln.Close()

	s := NewServer(port)
	if err := s.killByPort(port); err != nil {
		t.Errorf("killByPort on a free port returned %v, want nil", err)
	}
}
