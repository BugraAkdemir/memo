// SPDX-License-Identifier: AGPL-3.0-or-later

package llama

import (
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestParseListDevices(t *testing.T) {
	tests := []struct {
		name   string
		output string
		want   []string
		wantOK bool
	}{
		{
			name:   "no GPU backend loaded",
			output: "load_backend: loaded CPU backend from /x/libggml-cpu-haswell.so\nAvailable devices:\n  (none)\n",
			want:   nil,
			wantOK: true,
		},
		{
			name: "one Vulkan device, noise before the list",
			output: "ggml_vulkan: 0 = AMD Radeon Graphics (RADV RENOIR) (radv) | uma: 1 | fp16: 1\n" +
				"load_backend: loaded Vulkan backend from /x/libggml-vulkan.so\n" +
				"Available devices:\n  Vulkan0: AMD Radeon Graphics (RADV RENOIR) (8202 MiB, 7162 MiB free)\n",
			want:   []string{"Vulkan0"},
			wantOK: true,
		},
		{
			name:   "two devices",
			output: "Available devices:\n  CUDA0: NVIDIA GeForce RTX 3060 (12288 MiB, 11000 MiB free)\n  Vulkan1: NVIDIA GeForce RTX 3060 (12288 MiB, 11000 MiB free)\n",
			want:   []string{"CUDA0", "Vulkan1"},
			wantOK: true,
		},
		{
			name:   "an old build that does not know the flag has no list at all",
			output: "error: invalid argument: --list-devices\nusage: llama-server [options]\n",
			want:   nil,
			wantOK: false,
		},
		{
			name:   "empty output is no verdict",
			output: "",
			want:   nil,
			wantOK: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := parseListDevices(tt.output)
			if ok != tt.wantOK || !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("parseListDevices() = %v, %v; want %v, %v", got, ok, tt.want, tt.wantOK)
			}
		})
	}
}

// probeBinary writes a fake llama-server at root/binaries/<os>/<flavor> that
// answers --list-devices with devicesOut, and answers anything else the way a
// real build answers a probe run against a nonexistent model: it either got
// past argument parsing (acceptsFlags) and failed to load the model, or it
// rejected the arguments.
func probeBinary(t *testing.T, root, flavor, devicesOut string, acceptsFlags bool) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("fake shell binaries aren't directly executable on Windows")
	}
	p := filepath.Join(root, "binaries", runtime.GOOS, flavor, llamaServerBinary())
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	other := "echo 'error: invalid argument: --device'; exit 1"
	if acceptsFlags {
		other = "echo 'error loading model: failed to open __memo_tuning_probe_nonexistent__.gguf'; exit 1"
	}
	if err := os.WriteFile(filepath.Join(filepath.Dir(p), "libggml-vulkan.so"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	script := "#!/bin/sh\ncase \"$*\" in\n*--list-devices*)\ncat <<'EOF'\n" + devicesOut + "\nEOF\nexit 0;;\n*) " + other + ";;\nesac\n"
	if err := os.WriteFile(p, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return p
}

const (
	noGPU      = "Available devices:\n  (none)"
	vulkanGPU  = "Available devices:\n  Vulkan0: AMD Radeon Graphics (RADV RENOIR) (8202 MiB, 7162 MiB free)"
	hipGPU     = "Available devices:\n  ROCm0: AMD Radeon RX 7800 XT (16368 MiB, 16000 MiB free)"
	cudaGPU    = "Available devices:\n  CUDA0: NVIDIA GeForce RTX 3060 (12288 MiB, 11000 MiB free)"
	skipNotLnx = "device-based engine choice is Linux-only"
)

// withStubBackendLib drops an empty GPU backend library next to bin, as a real
// GPU-capable engine folder has, so the probe actually executes the binary.
func withStubBackendLib(t *testing.T, bin string) string {
	t.Helper()
	if err := os.WriteFile(filepath.Join(filepath.Dir(bin), "libggml-vulkan.so"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	return bin
}

func requireLinux(t *testing.T) {
	t.Helper()
	if runtime.GOOS != "linux" {
		t.Skip(skipNotLnx)
	}
}

func TestListGPUDevices_RunsTheBinary(t *testing.T) {
	root := t.TempDir()
	withGPU := probeBinary(t, root, "cpu", vulkanGPU, true)
	devs, ok := listGPUDevices(withGPU)
	if !ok || !reflect.DeepEqual(devs, []string{"Vulkan0"}) {
		t.Fatalf("got %v, %v; want [Vulkan0], true", devs, ok)
	}

	none := probeBinary(t, root, "amd", noGPU, true)
	devs, ok = listGPUDevices(none)
	if !ok || len(devs) != 0 {
		t.Fatalf("got %v, %v; want no devices, true", devs, ok)
	}
}

func TestListGPUDevices_ABinaryThatCannotRunIsNoVerdict(t *testing.T) {
	// The package's own fake engine is the literal byte "x": exec fails with
	// ENOEXEC, which says nothing about GPUs and must not be cached as an answer.
	root := t.TempDir()
	bin := withStubBackendLib(t, engine(t, root, "amd"))
	if devs, ok := listGPUDevices(bin); ok {
		t.Fatalf("got %v, true; an unrunnable binary must not conclude", devs)
	}
}

// A binary with no GPU backend library beside it cannot have a GPU backend, so
// it is not even run: a custom binary_path may be any program (the embedding
// lifecycle test's fake never exits), and a probe that waits for it would
// stall every start.
func TestListGPUDevices_NeverRunsABinaryWithoutAGPUBackendLibrary(t *testing.T) {
	requireLinux(t)
	dir := t.TempDir()
	marker := filepath.Join(dir, "was-run")
	bin := filepath.Join(dir, "llama-server")
	if err := os.WriteFile(bin, []byte("#!/bin/sh\ntouch '"+marker+"'\nsleep 30\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	if devs, ok := listGPUDevices(bin); ok {
		t.Fatalf("got %v, true; with no GPU backend library there is nothing to conclude", devs)
	}
	if _, err := os.Stat(marker); err == nil {
		t.Fatal("the binary was executed although no GPU backend library sits beside it")
	}
	if time.Since(start) > 2*time.Second {
		t.Fatalf("took %v; must return without waiting for the binary", time.Since(start))
	}
}

func TestResolveBinary_AutoSkipsAFlavorThatShowsNoGPU(t *testing.T) {
	requireLinux(t)
	_, bundled := withOverride(t)
	probeBinary(t, bundled, "amd", noGPU, true)
	want := probeBinary(t, bundled, "cpu", vulkanGPU, true)
	got, err := resolveBinary("", "auto")
	if err != nil || !samePath(t, got, want) {
		t.Fatalf("auto picked %q, %v; want the cpu engine %q (the amd one cannot see a GPU here)", got, err, want)
	}
}

func TestResolveBinary_AutoKeepsAFlavorThatShowsAGPU(t *testing.T) {
	requireLinux(t)
	_, bundled := withOverride(t)
	want := probeBinary(t, bundled, "amd", hipGPU, true)
	probeBinary(t, bundled, "cpu", vulkanGPU, true)
	got, err := resolveBinary("", "auto")
	if err != nil || !samePath(t, got, want) {
		t.Fatalf("auto picked %q, %v; want the amd engine %q", got, err, want)
	}
}

func TestResolveBinary_AutoPrefersTheFlavorThatSeesAGPUOverOneThatDoesNot(t *testing.T) {
	requireLinux(t)
	_, bundled := withOverride(t)
	probeBinary(t, bundled, "amd", noGPU, true)
	want := probeBinary(t, bundled, "nvidia", cudaGPU, true)
	probeBinary(t, bundled, "cpu", noGPU, true)
	got, err := resolveBinary("", "auto")
	if err != nil || !samePath(t, got, want) {
		t.Fatalf("auto picked %q, %v; want the nvidia engine %q", got, err, want)
	}
}

// A probe that gives no verdict (old build without --list-devices, a timeout)
// must leave the choice exactly as it was before this probe existed.
func TestResolveBinary_AutoKeepsTheOldOrderWhenTheProbeIsInconclusive(t *testing.T) {
	requireLinux(t)
	_, bundled := withOverride(t)
	want := withStubBackendLib(t, engine(t, bundled, "amd")) // unrunnable: no verdict
	probeBinary(t, bundled, "cpu", vulkanGPU, true)
	got, err := resolveBinary("", "auto")
	if err != nil || !samePath(t, got, want) {
		t.Fatalf("auto picked %q, %v; want the amd engine %q as before", got, err, want)
	}
}

func TestResolveBinary_AutoStillReturnsAGPULessFlavorWhenItIsTheOnlyOne(t *testing.T) {
	requireLinux(t)
	_, bundled := withOverride(t)
	want := probeBinary(t, bundled, "amd", noGPU, true)
	got, err := resolveBinary("", "auto")
	if err != nil || !samePath(t, got, want) {
		t.Fatalf("auto picked %q, %v; want the only engine there is, %q", got, err, want)
	}
}

func TestResolveBinary_ExplicitModeIgnoresTheDeviceProbe(t *testing.T) {
	requireLinux(t)
	_, bundled := withOverride(t)
	want := probeBinary(t, bundled, "amd", noGPU, true)
	probeBinary(t, bundled, "cpu", vulkanGPU, true)
	got, err := resolveBinary("", "amd")
	if err != nil || !samePath(t, got, want) {
		t.Fatalf("mode amd picked %q, %v; an explicit choice must be honoured: %q", got, err, want)
	}
}

func TestUsesVulkanOnlyGPU(t *testing.T) {
	requireLinux(t)
	cases := []struct {
		flavor, devices string
		want            bool
	}{
		{"cpu", vulkanGPU, true},   // the GPU comes from a plugin dropped into the cpu folder
		{"cpu", noGPU, false},      // plain CPU engine
		{"amd", hipGPU, false},     // the flavor's own GPU backend
		{"nvidia", cudaGPU, false}, // the flavor's own GPU backend
		// the b9441 nvidia folder carries Vulkan too: on a machine without CUDA that is all it sees
		{"nvidia", vulkanGPU, true},
		// a native device next to a Vulkan one: the native backend decides
		{"nvidia", cudaGPU + "\n  Vulkan1: NVIDIA GeForce RTX 3060 (12288 MiB, 11000 MiB free)", false},
	}
	for _, c := range cases {
		bin := probeBinary(t, t.TempDir(), c.flavor, c.devices, true)
		if got := usesVulkanOnlyGPU(bin); got != c.want {
			t.Errorf("usesVulkanOnlyGPU(%s flavor, devices %q) = %v, want %v", c.flavor, c.devices, got, c.want)
		}
	}
	if usesVulkanOnlyGPU(withStubBackendLib(t, engine(t, t.TempDir(), "cpu"))) {
		t.Error("a cpu engine that cannot run must not count as a GPU engine")
	}
}

func TestHostOnlyArgs(t *testing.T) {
	requireLinux(t)
	pluginCPU := probeBinary(t, t.TempDir(), "cpu", vulkanGPU, true)
	plainCPU := probeBinary(t, t.TempDir(), "cpu", noGPU, true)
	amd := probeBinary(t, t.TempDir(), "amd", hipGPU, true)
	oldCPU := probeBinary(t, t.TempDir(), "cpu", vulkanGPU, false) // rejects --device
	nvidiaVulkan := probeBinary(t, t.TempDir(), "nvidia", vulkanGPU, true)
	devNone := []string{"--device", "none"}

	tests := []struct {
		name      string
		bin       string
		mode      string
		embedding bool
		gpu       int
		rpc       bool
		want      []string
	}{
		{"embedding sidecar on a plugin GPU stays on the host: the GPU is slower for tiny batches and changes the vectors", pluginCPU, "auto", true, 0, false, devNone},
		{"embedding with explicit GPU layers is left alone", pluginCPU, "auto", true, 20, false, nil},
		{"a chat model on the plugin GPU is the whole point", pluginCPU, "auto", false, 0, false, nil},
		{"engine_mode cpu means no GPU even when a plugin is present", pluginCPU, "cpu", false, 0, false, devNone},
		{"engine_mode cpu on a plain cpu engine needs no flag", plainCPU, "cpu", false, 0, false, nil},
		{"engine_mode cpu never rewrites a GPU flavor's own backend, only the plugin case was measured", amd, "cpu", false, 0, false, nil},
		{"embedding on a plain cpu engine needs no flag", plainCPU, "auto", true, 0, false, nil},
		{"the amd engine's own GPU backend is not touched", amd, "auto", true, 0, false, nil},
		{"a Vulkan-only GPU is host-only for embedding whatever the folder is called", nvidiaVulkan, "auto", true, 0, false, devNone},
		{"a build that rejects --device is never given it", oldCPU, "cpu", false, 0, false, nil},
		{"the swarm coordinator keeps its RPC devices", pluginCPU, "cpu", false, 0, true, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := hostOnlyArgs(tt.bin, tt.mode, tt.embedding, tt.gpu, tt.rpc)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("hostOnlyArgs() = %v, want %v", got, tt.want)
			}
		})
	}
}

// dynamicBinary is a fake engine whose answer follows marker files, so a test
// can change what the "machine" shows without touching the binary (whose mtime
// is part of the cache key).
func dynamicBinary(t *testing.T, dir, gpuMarker string) string {
	t.Helper()
	requireLinux(t)
	bin := filepath.Join(dir, "llama-server")
	script := "#!/bin/sh\nif [ -e '" + gpuMarker + "' ]; then\nprintf 'Available devices:\\n  Vulkan0: GPU (8000 MiB, 7000 MiB free)\\n'\nelse\nprintf 'Available devices:\\n  (none)\\n'\nfi\n"
	if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return withStubBackendLib(t, bin)
}

func fakeClock(t *testing.T) *time.Time {
	t.Helper()
	cur := time.Now()
	prev := probeNow
	probeNow = func() time.Time { return cur }
	t.Cleanup(func() { probeNow = prev })
	return &cur
}

// "No GPU here" is a snapshot of driver state, not a property of the binary: a
// driver that is not ready at autostart, a hybrid laptop's dGPU that is off, a
// CUDA context that failed after suspend. It must not pin the machine to the
// CPU engine for the rest of the process.
func TestListGPUDevices_ANoGPUVerdictExpires(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "gpu-ready") // outside the engine folder: its mtime is part of the key
	bin := dynamicBinary(t, t.TempDir(), marker)
	clock := fakeClock(t)

	if devs, ok := listGPUDevices(bin); !ok || len(devs) != 0 {
		t.Fatalf("first probe = %v, %v; want no devices", devs, ok)
	}
	if err := os.WriteFile(marker, nil, 0o644); err != nil { // the GPU comes up
		t.Fatal(err)
	}
	if devs, _ := listGPUDevices(bin); len(devs) != 0 {
		t.Fatalf("a fresh no-GPU verdict should still be remembered, got %v", devs)
	}
	*clock = clock.Add(deviceNegativeTTL + time.Second)
	if devs, ok := listGPUDevices(bin); !ok || len(devs) != 1 {
		t.Fatalf("after the TTL the machine must be asked again, got %v, %v", devs, ok)
	}
}

func TestListGPUDevices_AFoundGPUIsRememberedWithoutAskingAgain(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "gpu-ready")
	if err := os.WriteFile(marker, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	bin := dynamicBinary(t, t.TempDir(), marker)
	clock := fakeClock(t)

	if devs, _ := listGPUDevices(bin); len(devs) != 1 {
		t.Fatalf("got %v, want the GPU", devs)
	}
	os.Remove(marker)
	*clock = clock.Add(10 * deviceNegativeTTL)
	if devs, _ := listGPUDevices(bin); len(devs) != 1 {
		t.Fatalf("a GPU that was found is the stable answer, got %v", devs)
	}
}

// Dropping a backend plugin into (or removing one from) the engine folder
// changes what the engine can see without touching llama-server itself.
func TestListGPUDevices_ChangingTheEngineFolderInvalidatesTheVerdict(t *testing.T) {
	dir := t.TempDir()
	marker := filepath.Join(t.TempDir(), "gpu-ready")
	bin := dynamicBinary(t, dir, marker)

	if devs, ok := listGPUDevices(bin); !ok || len(devs) != 0 {
		t.Fatalf("first probe = %v, %v", devs, ok)
	}
	time.Sleep(30 * time.Millisecond)                        // directory mtime granularity
	if err := os.WriteFile(marker, nil, 0o644); err != nil { // the GPU is there now, the engine folder is not yet changed
		t.Fatal(err)
	}
	if devs, _ := listGPUDevices(bin); len(devs) != 0 {
		t.Fatalf("an unchanged engine folder keeps its remembered answer, got %v", devs)
	}
	if err := os.WriteFile(filepath.Join(dir, "libggml-hip.so"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if devs, _ := listGPUDevices(bin); len(devs) != 1 {
		t.Fatalf("a changed engine folder must be probed again, got %v", devs)
	}
}

func TestListGPUDevices_ATimedOutProbeIsNotRetriedOnEveryStart(t *testing.T) {
	requireLinux(t)
	dir := t.TempDir()
	bin := filepath.Join(dir, "llama-server")
	if err := os.WriteFile(bin, []byte("#!/bin/sh\nsleep 30\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	withStubBackendLib(t, bin)
	prev := probeTimeout
	probeTimeout = 300 * time.Millisecond
	t.Cleanup(func() { probeTimeout = prev })

	if _, ok := listGPUDevices(bin); ok {
		t.Fatal("a hung binary must not conclude")
	}
	start := time.Now()
	if _, ok := listGPUDevices(bin); ok || time.Since(start) > 100*time.Millisecond {
		t.Fatalf("the second start must not wait for the same hang again (took %v)", time.Since(start))
	}
}

// resolveBinary probes the copy it would actually run: a downloaded engine
// shadows the bundled one of the same flavor.
func TestResolveBinary_AutoProbesTheDownloadedCopyOfAFlavor(t *testing.T) {
	requireLinux(t)
	over, bundled := withOverride(t)
	probeBinary(t, bundled, "amd", hipGPU, true) // the bundled one would see a GPU
	probeBinary(t, over, "amd", noGPU, true)     // the downloaded one is what runs, and does not
	want := probeBinary(t, bundled, "cpu", vulkanGPU, true)
	got, err := resolveBinary("", "auto")
	if err != nil || !samePath(t, got, want) {
		t.Fatalf("auto picked %q, %v; want the cpu engine %q", got, err, want)
	}
}

func TestParseListDevices_CRLFAndTrailingNoise(t *testing.T) {
	out := "Available devices:\r\n  Vulkan0: GPU (8000 MiB, 7000 MiB free)\r\n\r\nload_backend: loaded CPU backend\r\n"
	got, ok := parseListDevices(out)
	if !ok || !reflect.DeepEqual(got, []string{"Vulkan0"}) {
		t.Fatalf("got %v, %v", got, ok)
	}
	if strings.Contains(strings.Join(got, ""), "load_backend") {
		t.Fatal("noise after the list must not be read as a device")
	}
}

// The real bundle ships amd + nvidia + cpu side by side, and the nvidia folder
// (b9441) carries a Vulkan backend as well. On a machine with neither ROCm nor
// CUDA support that folder sees only the Vulkan GPU: not what the folder is for,
// and an older build than cpu + plugin.
func TestResolveBinary_AutoTreatsAVulkanOnlyGPUFlavorAsNotNative(t *testing.T) {
	requireLinux(t)

	t.Run("cpu with the plugin wins over a GPU flavor that only sees Vulkan", func(t *testing.T) {
		_, bundled := withOverride(t)
		probeBinary(t, bundled, "amd", noGPU, true)
		probeBinary(t, bundled, "nvidia", vulkanGPU, true)
		want := probeBinary(t, bundled, "cpu", vulkanGPU, true)
		got, err := resolveBinary("", "auto")
		if err != nil || !samePath(t, got, want) {
			t.Fatalf("auto picked %q, %v; want the cpu engine %q", got, err, want)
		}
	})

	t.Run("without the plugin, the flavor that sees Vulkan is still better than plain cpu", func(t *testing.T) {
		_, bundled := withOverride(t)
		probeBinary(t, bundled, "amd", noGPU, true)
		want := probeBinary(t, bundled, "nvidia", vulkanGPU, true)
		probeBinary(t, bundled, "cpu", noGPU, true)
		got, err := resolveBinary("", "auto")
		if err != nil || !samePath(t, got, want) {
			t.Fatalf("auto picked %q, %v; want the nvidia engine %q", got, err, want)
		}
	})

	t.Run("a native device keeps its flavor even when cpu has the plugin", func(t *testing.T) {
		_, bundled := withOverride(t)
		probeBinary(t, bundled, "amd", noGPU, true)
		want := probeBinary(t, bundled, "nvidia", cudaGPU+"\n  Vulkan1: NVIDIA GeForce RTX 3060 (12288 MiB, 11000 MiB free)", true)
		probeBinary(t, bundled, "cpu", vulkanGPU, true)
		got, err := resolveBinary("", "auto")
		if err != nil || !samePath(t, got, want) {
			t.Fatalf("auto picked %q, %v; want the nvidia engine %q", got, err, want)
		}
	})

	t.Run("nothing sees any GPU: the cpu engine, the newest build", func(t *testing.T) {
		_, bundled := withOverride(t)
		probeBinary(t, bundled, "amd", noGPU, true)
		probeBinary(t, bundled, "nvidia", noGPU, true)
		want := probeBinary(t, bundled, "cpu", noGPU, true)
		got, err := resolveBinary("", "auto")
		if err != nil || !samePath(t, got, want) {
			t.Fatalf("auto picked %q, %v; want the cpu engine %q", got, err, want)
		}
	})
}
