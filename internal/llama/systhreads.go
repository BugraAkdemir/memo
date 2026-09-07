package llama

import (
	"bufio"
	"io"
	"strings"
)

// parseCPUInfoPhysicalCores counts distinct physical CPU cores from the
// contents of a Linux /proc/cpuinfo stream: one entry per unique
// (physical id, core id) pair. Returns 0 when those fields are absent (some
// ARM kernels omit them) — the caller then falls back to not tuning threads
// at all. Split out from physicalCoreCount so it can be unit tested without
// a real /proc.
func parseCPUInfoPhysicalCores(r io.Reader) int {
	seen := make(map[string]struct{})
	var physID, coreID string
	haveCore := false

	flush := func() {
		if haveCore {
			seen[physID+"/"+coreID] = struct{}{}
		}
		physID, coreID, haveCore = "", "", false
	}

	sc := bufio.NewScanner(r)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" { // blank line separates logical-CPU records
			flush()
			continue
		}
		key, val, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		switch strings.TrimSpace(key) {
		case "physical id":
			physID = strings.TrimSpace(val)
		case "core id":
			coreID = strings.TrimSpace(val)
			haveCore = true
		}
	}
	flush()
	return len(seen)
}

// serverThreads decides the value for llama-server's --threads flag, given
// the GPU-layer count actually in effect plus the host's logical and
// physical core counts. Returns 0 to mean "pass no --threads, let
// llama.cpp's own default stand".
//
//   - GPU offload active  -> 0. The CPU does almost no work; llama.cpp's
//     default is fine and a wrong number here only adds scheduling overhead.
//   - CPU inference, SMT present (physical < logical) -> physical. For a
//     compute-bound decode, physical cores beat hyperthreads: the sibling
//     threads just contend for the same execution units. llama.cpp's
//     default is std::thread::hardware_concurrency() == logical, which
//     over-subscribes here.
//   - CPU inference, no SMT or physical count unknown -> 0. The default
//     already equals the physical core count, nothing to fix.
func serverThreads(gpuLayers, logical, physical int) int {
	if gpuLayers > 0 {
		return 0
	}
	if physical > 0 && physical < logical {
		return physical
	}
	return 0
}
