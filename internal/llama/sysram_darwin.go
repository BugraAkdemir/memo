//go:build darwin

package llama

import (
	"os/exec"
	"strconv"
	"strings"
)

// systemRAMMb returns total physical RAM in MB via `sysctl -n hw.memsize`,
// or 0 if it can't be determined.
func systemRAMMb() int {
	out, err := exec.Command("sysctl", "-n", "hw.memsize").Output()
	if err != nil {
		return 0
	}
	bytes, err := strconv.ParseInt(strings.TrimSpace(string(out)), 10, 64)
	if err != nil {
		return 0
	}
	return int(bytes / (1024 * 1024))
}
