//go:build linux

package whisper

import (
	"net"
	"os"
	"path/filepath"
	"testing"
)

// realisticProcNetTCPLine mirrors an actual /proc/net/tcp entry, with the
// local port and state substituted. Field 9 (index) is the inode.
func procNetTCPFixture(t *testing.T, localAddrPort, state, inode string) string {
	t.Helper()
	header := "  sl  local_address rem_address   st tx_queue rx_queue tr tm->when retrnsmt   uid  timeout inode\n"
	line := "   0: " + localAddrPort + " 00000000:0000 " + state +
		" 00000000:00000000 00:00000000 00000000  1000        0 " + inode +
		" 1 0000000000000000 100 0 0 10 0\n"
	return header + line
}

func writeFixture(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "fake_proc_net_tcp")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	return path
}

func TestScanProcNetTCP_FindsListeningPort(t *testing.T) {
	// port 8080 = 0x1F90
	path := writeFixture(t, procNetTCPFixture(t, "0100007F:1F90", tcpListen, "12345"))

	inode := scanProcNetTCP(path, 8080)
	if inode != "12345" {
		t.Errorf("scanProcNetTCP() = %q, want %q", inode, "12345")
	}
}

func TestScanProcNetTCP_IgnoresNonListenState(t *testing.T) {
	// state 06 == TCP_CLOSE, not LISTEN — must not match even though the
	// port is right, exactly the distinction that matters for correctly
	// telling "someone is listening" from "there was a connection here".
	path := writeFixture(t, procNetTCPFixture(t, "0100007F:1F90", "06", "12345"))

	if inode := scanProcNetTCP(path, 8080); inode != "" {
		t.Errorf("scanProcNetTCP() = %q for a non-LISTEN socket, want \"\"", inode)
	}
}

func TestScanProcNetTCP_WrongPortNotMatched(t *testing.T) {
	path := writeFixture(t, procNetTCPFixture(t, "0100007F:1F90", tcpListen, "12345"))

	if inode := scanProcNetTCP(path, 9090); inode != "" {
		t.Errorf("scanProcNetTCP() = %q for an unrelated port, want \"\"", inode)
	}
}

func TestScanProcNetTCP_IPv6AddressFormat(t *testing.T) {
	// /proc/net/tcp6 uses a 32-hex-char address instead of tcp's 8, but the
	// ":PORT" suffix and field layout are identical — the parser must not
	// assume a fixed address width.
	path := writeFixture(t, procNetTCPFixture(t, "00000000000000000000000000000000:1F90", tcpListen, "999"))

	if inode := scanProcNetTCP(path, 8080); inode != "999" {
		t.Errorf("scanProcNetTCP() = %q, want %q for an IPv6-shaped address", inode, "999")
	}
}

func TestScanProcNetTCP_MalformedLineSkippedNotPanicked(t *testing.T) {
	content := "  sl  local_address rem_address   st tx_queue rx_queue tr tm->when retrnsmt   uid  timeout inode\n" +
		"   0: garbage\n" +
		"   1: notacolonaddr 00000000:0000 0A 00000000:00000000 00:00000000 00000000 1000 0 42 1 0 0 0 0 0\n"
	path := writeFixture(t, content)

	if inode := scanProcNetTCP(path, 8080); inode != "" {
		t.Errorf("scanProcNetTCP() = %q on malformed input, want \"\" (and no panic)", inode)
	}
}

func TestScanProcNetTCP_MissingFileReturnsEmpty(t *testing.T) {
	if inode := scanProcNetTCP(filepath.Join(t.TempDir(), "does-not-exist"), 8080); inode != "" {
		t.Errorf("scanProcNetTCP() = %q for a missing file, want \"\"", inode)
	}
}

// TestFindPidForInode_MatchesRealListener isolates findPidForInode from
// scanProcNetTCP (already covered end-to-end by TestPidListeningOnPort_*
// equivalents) — it opens a real listening socket, reads its true inode out
// of the live /proc/net/tcp, and confirms the fd-table walk attributes it
// back to this test process's own PID.
func TestFindPidForInode_MatchesRealListener(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()
	port := ln.Addr().(*net.TCPAddr).Port

	inode := findListenInode(port)
	if inode == "" {
		t.Skip("could not read own listener's inode from /proc/net/tcp in this environment")
	}

	if got := findPidForInode(inode); got != os.Getpid() {
		t.Errorf("findPidForInode(%q) = %d, want this process (%d)", inode, got, os.Getpid())
	}
}
