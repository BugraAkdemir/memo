//go:build darwin

package llama

import (
	"fmt"
	"memo/internal/logx"
	"os/exec"
	"strings"
)

// pidListeningOnPort shells out to lsof (primary) or fuser (fallback) to
// find the PID of the process bound to port. macOS has no /proc, so unlike
// Linux (see process_linux.go) there is no dependency-free native path —
// but lsof ships with the OS itself (part of the base BSD userland), so the
// "neither tool installed" failure mode that motivated the Linux rewrite is
// not a realistic risk here.
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
