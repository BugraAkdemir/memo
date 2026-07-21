//go:build linux

package llama

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// tcpListen is the "st" field value /proc/net/tcp{,6} use for a socket in
// LISTEN state (see linux/include/net/tcp_states.h — TCP_LISTEN == 10 == 0xA).
const tcpListen = "0A"

// pidListeningOnPort finds the PID bound to port by reading /proc directly
// instead of shelling out to lsof/fuser. Both tools are optional packages
// (lsof, psmisc) that are frequently missing from minimal Linux installs and
// containers — on a machine with neither installed, the old lsof/fuser-based
// implementation always returned 0 ("port free"), silently disabling orphan
// port cleanup entirely (see the whisper-server orphan-port bug, 2026-07-20).
// procfs, by contrast, is essentially guaranteed present on any Linux system
// capable of running Memo's subprocesses at all.
func pidListeningOnPort(port int) int {
	inode := findListenInode(port)
	if inode == "" {
		return 0
	}
	return findPidForInode(inode)
}

// findListenInode scans /proc/net/tcp and /proc/net/tcp6 for a socket in
// LISTEN state bound to port, returning its inode number (as a string, to
// match directly against the "socket:[N]" fd symlink target).
func findListenInode(port int) string {
	for _, path := range []string{"/proc/net/tcp", "/proc/net/tcp6"} {
		if inode := scanProcNetTCP(path, port); inode != "" {
			return inode
		}
	}
	return ""
}

func scanProcNetTCP(path string, port int) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	scanner.Scan() // header line ("sl local_address rem_address st ...")
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		// sl local_address rem_address st tx_queue:rx_queue tr:tm->when
		// retrnsmt uid timeout inode
		if len(fields) < 10 {
			continue
		}
		if fields[3] != tcpListen {
			continue
		}
		addrParts := strings.Split(fields[1], ":")
		if len(addrParts) != 2 {
			continue
		}
		localPort, err := strconv.ParseInt(addrParts[1], 16, 32)
		if err != nil || int(localPort) != port {
			continue
		}
		return fields[9]
	}
	return ""
}

// findPidForInode walks /proc/<pid>/fd/* looking for a symlink whose target
// is "socket:[inode]" — the kernel-provided reverse mapping from an open
// socket back to the process holding it.
func findPidForInode(inode string) int {
	target := fmt.Sprintf("socket:[%s]", inode)

	entries, err := os.ReadDir("/proc")
	if err != nil {
		return 0
	}
	for _, entry := range entries {
		pid, err := strconv.Atoi(entry.Name())
		if err != nil {
			continue // not a PID directory (self, cpuinfo, etc.)
		}

		fdDir := filepath.Join("/proc", entry.Name(), "fd")
		fds, err := os.ReadDir(fdDir)
		if err != nil {
			continue // process exited mid-scan, or fd dir unreadable — skip
		}
		for _, fd := range fds {
			link, err := os.Readlink(filepath.Join(fdDir, fd.Name()))
			if err != nil {
				continue
			}
			if link == target {
				return pid
			}
		}
	}
	return 0
}
