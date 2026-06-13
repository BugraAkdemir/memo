//go:build linux

package llama

import (
	"bufio"
	"os"
	"strconv"
	"strings"
)

// systemRAMMb returns total physical RAM in MB by reading /proc/meminfo,
// or 0 if it can't be determined.
func systemRAMMb() int {
	f, err := os.Open("/proc/meminfo")
	if err != nil {
		return 0
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "MemTotal:") {
			continue
		}
		fields := strings.Fields(line) // ["MemTotal:", "16313456", "kB"]
		if len(fields) < 2 {
			return 0
		}
		kb, err := strconv.Atoi(fields[1])
		if err != nil {
			return 0
		}
		return kb / 1024
	}
	return 0
}
