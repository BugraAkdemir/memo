// SPDX-License-Identifier: AGPL-3.0-or-later

package llama

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"

	"memo/internal/logx"
)

// deviceLine matches one entry of `llama-server --list-devices`, e.g.
//
//	Vulkan0: AMD Radeon Graphics (RADV RENOIR) (8202 MiB, 7162 MiB free)
var deviceLine = regexp.MustCompile(`^\s+([A-Za-z][A-Za-z0-9_]*\d+):\s`)

// parseListDevices extracts the GPU device names from the output of
// `llama-server --list-devices`. ok is false when the output has no
// "Available devices:" header at all (a build that predates the flag, or a
// binary that never got that far) — that is no verdict, whereas a header with
// "(none)" under it is a definite "this build sees no GPU".
func parseListDevices(output string) (devs []string, ok bool) {
	inList := false
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimRight(line, "\r")
		if !inList {
			if strings.HasPrefix(strings.TrimSpace(line), "Available devices:") {
				inList = true
			}
			continue
		}
		if m := deviceLine.FindStringSubmatch(line); m != nil {
			devs = append(devs, m[1])
		}
	}
	return devs, inList
}

// probeTimeout bounds one --list-devices run. A real build answers in tens of
// milliseconds (15-70 ms measured), so this only ever bites a binary that is
// not a llama-server, or a GPU runtime that hangs while initialising. A var so
// tests can shorten it.
var probeTimeout = 5 * time.Second

// deviceNegativeTTL is how long a probe answer that is NOT "a GPU was found"
// (no GPU listed, or no verdict at all, e.g. a timeout) is remembered. Neither
// is a property of the binary: a driver that is not ready at autostart, a
// hybrid laptop's dGPU that is off or a CUDA context lost after suspend all
// look like "none" for a while, and remembering that for the whole process
// would pin the machine to the CPU engine until Memo restarts. It is also
// what keeps a hung probe from costing probeTimeout on every start.
const deviceNegativeTTL = 60 * time.Second

// probeNow is the clock the cache expires by; a var so tests can move it.
var probeNow = time.Now

// gpuBackendLibs are the file patterns of ggml GPU backend libraries. ggml loads
// every one it finds next to llama-server, so their presence is what makes an
// engine folder capable of a GPU at all.
var gpuBackendLibs = []string{
	"libggml-vulkan*", "libggml-cuda*", "libggml-hip*", "libggml-sycl*", "libggml-opencl*", "libggml-metal*",
	"ggml-vulkan*.dll", "ggml-cuda*.dll", "ggml-hip*.dll", "ggml-sycl*.dll", "ggml-opencl*.dll",
}

func hasGPUBackendLib(bin string) bool {
	dir := filepath.Dir(bin)
	for _, pat := range gpuBackendLibs {
		if m, _ := filepath.Glob(filepath.Join(dir, pat)); len(m) > 0 {
			return true
		}
	}
	return false
}

// deviceAnswer is one remembered --list-devices result. A found GPU is kept
// until the key changes; anything else expires at `until`.
type deviceAnswer struct {
	devs      []string
	concluded bool
	until     time.Time // zero = does not expire
}

// deviceCache maps probeCacheKey to a deviceAnswer.
var deviceCache sync.Map // map[string]deviceAnswer

func probeCacheKey(bin string) string {
	abs, err := filepath.Abs(bin)
	if err != nil {
		abs = bin
	}
	key := abs
	if fi, statErr := os.Stat(abs); statErr == nil {
		key += "@" + fi.ModTime().UTC().Format(time.RFC3339Nano)
	}
	// What an engine can see also depends on the backend libraries beside it:
	// dropping libggml-vulkan.so into (or removing it from) the folder leaves
	// llama-server's own mtime untouched but changes the folder's.
	if fi, statErr := os.Stat(filepath.Dir(abs)); statErr == nil {
		key += "#" + fi.ModTime().UTC().Format(time.RFC3339Nano)
	}
	return key
}

// listGPUDevices asks bin which GPU devices it can use. A bundled engine
// folder does not always see the GPU it was built for (the linux/amd folder is
// a ROCm build, and ROCm supports only some cards), and a cpu folder gains one
// when a Vulkan backend plugin sits next to it, so the only reliable answer is
// to ask the build itself. concluded is false when the run produced no verdict
// (timeout, binary that cannot start, a build without --list-devices); callers
// must then behave as if the probe did not exist.
//
// Linux only: the Windows and macOS engines cannot be run from here to confirm
// what they print, so there the probe gives no verdict and everything keeps the
// behaviour it always had.
func listGPUDevices(bin string) (devs []string, concluded bool) {
	if runtime.GOOS != "linux" {
		return nil, false
	}
	key := probeCacheKey(bin)
	if v, ok := deviceCache.Load(key); ok {
		if a := v.(deviceAnswer); a.until.IsZero() || probeNow().Before(a.until) {
			return a.devs, a.concluded
		}
	}
	answer := func(devs []string, concluded bool) ([]string, bool) {
		a := deviceAnswer{devs: devs, concluded: concluded}
		if len(devs) == 0 {
			a.until = probeNow().Add(deviceNegativeTTL)
		}
		deviceCache.Store(key, a)
		return devs, concluded
	}
	// No GPU backend library beside it, nothing to ask: a custom binary_path can
	// be any program, and running it with a flag it does not know is not free.
	if !hasGPUBackendLib(bin) {
		return nil, false
	}
	ctx, cancel := context.WithTimeout(context.Background(), probeTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, "--list-devices")
	cmd.Env = engineEnv(bin)
	// CommandContext only kills the process it started; a child that inherited
	// the output pipe would otherwise keep CombinedOutput waiting long past the
	// deadline.
	cmd.WaitDelay = 500 * time.Millisecond
	out, _ := cmd.CombinedOutput()
	if ctx.Err() != nil {
		logx.Printf("llama: device probe of %s timed out; choosing the engine without it", filepath.Base(filepath.Dir(bin)))
		return answer(nil, false)
	}
	devs, ok := parseListDevices(string(out))
	if !ok {
		return answer(nil, false)
	}
	return answer(devs, true)
}

// onlyVulkan reports whether every listed device is a Vulkan one. Vulkan is not
// what the amd (ROCm) and nvidia (CUDA) folders exist for, and on a machine
// those cannot drive it is all they see: the b9441 nvidia folder also carries a
// Vulkan backend and lists the integrated GPU of a plain AMD/Intel laptop.
func onlyVulkan(devs []string) bool {
	for _, d := range devs {
		if !strings.HasPrefix(d, "Vulkan") {
			return false
		}
	}
	return len(devs) > 0
}

// usesVulkanOnlyGPU reports whether bin's only GPU is a Vulkan one, whichever
// folder it lives in (a Vulkan backend plugin dropped into the cpu folder, or
// the Vulkan backend of the nvidia folder on a machine without CUDA). Such a GPU
// is used for prompt processing even with zero offloaded layers, and Memo's own
// GPU detection (sysfs/nvidia-smi) knows nothing about it, so it must not size
// layer offload from that detection.
func usesVulkanOnlyGPU(bin string) bool {
	devs, ok := listGPUDevices(bin)
	return ok && onlyVulkan(devs)
}

// gpuSelector implements resolveBinary's auto choice among the bundled flavors,
// visited in the order amd, nvidia, cpu. The old rule — first folder that exists —
// ran everything on the CPU whenever the amd (ROCm) folder was present on a
// machine ROCm does not support, however capable the others were.
//
//   - a flavor that lists a native (non-Vulkan) device is used right away;
//   - one that lists nothing, or only Vulkan, is remembered and skipped for now;
//   - the cpu flavor is used if it lists a device (a Vulkan plugin sits beside
//     it; it is also the newest build), otherwise the remembered Vulkan flavor,
//     otherwise cpu;
//   - a probe with no verdict (older build, hung runtime, Windows/macOS) leaves
//     the old answer: that folder, at once.
type gpuSelector struct{ none, vulkan string }

// consider returns the engine to use when p (a bundled copy of flavor) settles
// the choice, or done=false to keep looking.
func (g *gpuSelector) consider(flavor, p string) (choice string, done bool) {
	devs, ok := listGPUDevices(p)
	if flavor == "cpu" {
		if ok && len(devs) > 0 {
			return p, true
		}
		if g.vulkan != "" {
			return g.vulkan, true
		}
		return p, true
	}
	switch {
	case !ok:
		return p, true
	case len(devs) == 0:
		if g.none == "" {
			g.none = p
		}
		return "", false
	case onlyVulkan(devs):
		if g.vulkan == "" {
			g.vulkan = p
		}
		return "", false
	}
	return p, true
}

// fallback is the answer when no cpu flavor ended the walk: rather one of the
// skipped flavors than none at all.
func (g *gpuSelector) fallback() string {
	if g.vulkan != "" {
		return g.vulkan
	}
	return g.none
}

// deviceNoneArgs are the arguments that keep a llama-server on the host CPU
// even though a GPU backend is loaded.
var deviceNoneArgs = []string{"--device", "none"}

var deviceFlagCache sync.Map // map[string]bool

// deviceNoneSupported probes whether this build accepts `--device none`.
func deviceNoneSupported(bin string) bool {
	return flagsAccepted(&deviceFlagCache, bin, deviceNoneArgs)
}

// hostOnlyArgs returns `--device none` for the two cases that must stay off a
// GPU backend even though one is loaded, or nil:
//
//   - engine_mode "cpu": the user asked for no GPU, and a Vulkan plugin in the
//     cpu folder would otherwise take it anyway. Only a Vulkan-only GPU is
//     rewritten; what a native (ROCm/CUDA) backend does under "cpu" is unchanged.
//   - the embedding sidecar on a Vulkan-only GPU with no layers requested: measured
//     on Vulkan (RADV RENOIR) it is ~10x slower for short inputs and its
//     vectors differ from the CPU ones (cosine 0.991-0.994), so stored and new
//     vectors would no longer share one space.
//
// The swarm coordinator (rpc) keeps its devices. A build that rejects the flag
// is never given it.
func hostOnlyArgs(bin, mode string, embedding bool, gpuLayers int, rpc bool) []string {
	if rpc {
		return nil
	}
	want := (mode == "cpu" || (embedding && gpuLayers == 0)) && usesVulkanOnlyGPU(bin)
	if !want || !deviceNoneSupported(bin) {
		return nil
	}
	return deviceNoneArgs
}
