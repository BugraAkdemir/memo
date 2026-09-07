//go:build linux

package llama

import "os"

// physicalCoreCount returns the host's physical CPU core count from
// /proc/cpuinfo, or 0 if it can't be determined.
func physicalCoreCount() int {
	f, err := os.Open("/proc/cpuinfo")
	if err != nil {
		return 0
	}
	defer f.Close()
	return parseCPUInfoPhysicalCores(f)
}
