package llama

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// writeFakeShellBinary writes an executable shell script to dir/name that
// echoes body to stdout and exits with code, returning its path. Skips the
// test on Windows, where a plain #!/bin/sh script isn't directly
// executable — probeRPCSupport's own exec.CommandContext call has no
// Windows-specific behavior worth testing separately here.
func writeFakeShellBinary(t *testing.T, dir, name, body string, code int) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("fake shell binaries aren't directly executable on Windows")
	}
	path := filepath.Join(dir, name)
	script := "#!/bin/sh\necho '" + body + "'\nexit " + itoa(code) + "\n"
	if err := os.WriteFile(path, []byte(script), 0755); err != nil {
		t.Fatalf("write fake binary: %v", err)
	}
	return path
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var digits []byte
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	if neg {
		digits = append([]byte{'-'}, digits...)
	}
	return string(digits)
}

func TestProbeRPCSupport_InvalidArgumentMeansUnsupported(t *testing.T) {
	bin := writeFakeShellBinary(t, t.TempDir(), "llama-server", "error: invalid argument: --rpc", 1)
	if probeRPCSupport(bin) {
		t.Errorf("probeRPCSupport(%q) = true, want false for a build that rejects --rpc", bin)
	}
}

func TestProbeRPCSupport_PastArgParsingMeansSupported(t *testing.T) {
	bin := writeFakeShellBinary(t, t.TempDir(), "llama-server", "error: could not load model file", 1)
	if !probeRPCSupport(bin) {
		t.Errorf("probeRPCSupport(%q) = false, want true — this build got past arg-parsing, so --rpc itself was accepted", bin)
	}
}

func TestProbeRPCSupport_CachesPerBinaryPath(t *testing.T) {
	dir := t.TempDir()
	// The script overwrites itself on first run to a form that would flip
	// the answer if actually re-executed — a second probeRPCSupport call
	// returning the same (now-stale) result proves the cache, not a fresh
	// exec, served it.
	bin := filepath.Join(dir, "llama-server")
	script := "#!/bin/sh\n" +
		"echo 'error: invalid argument: --rpc'\n" +
		"echo '#!/bin/sh\\necho ok\\nexit 0' > '" + bin + "'\n" +
		"chmod +x '" + bin + "'\n" +
		"exit 1\n"
	if runtime.GOOS == "windows" {
		t.Skip("fake shell binaries aren't directly executable on Windows")
	}
	if err := os.WriteFile(bin, []byte(script), 0755); err != nil {
		t.Fatalf("write fake binary: %v", err)
	}

	first := probeRPCSupport(bin)
	if first {
		t.Fatalf("first probeRPCSupport(%q) = true, want false", bin)
	}
	second := probeRPCSupport(bin)
	if second != first {
		t.Errorf("second probeRPCSupport(%q) = %v, want %v (cached) — binary was rewritten between calls, so a differing result means the cache was bypassed", bin, second, first)
	}
}

func TestBuildRPCArgs_AlwaysForcesLayerSplitMode(t *testing.T) {
	args := buildRPCArgs(RPCOptions{
		Servers:     []string{"192.168.1.10:50052", "192.168.1.11:50052"},
		TensorSplit: []float64{40, 30, 30},
	})

	joined := strings.Join(args, " ")
	if !strings.Contains(joined, "--split-mode layer") {
		t.Errorf("buildRPCArgs() = %q, want it to contain \"--split-mode layer\"", joined)
	}
	if strings.Contains(joined, "row") {
		t.Errorf("buildRPCArgs() = %q, must never contain \"row\" as a split-mode value (needs interconnect far tighter than a home network)", joined)
	}
}

func TestBuildRPCArgs_TensorSplitAndRPCPositionallyAligned(t *testing.T) {
	args := buildRPCArgs(RPCOptions{
		Servers:     []string{"host1:50052", "host2:50052"},
		TensorSplit: []float64{50, 25, 25},
	})

	want := []string{
		"--split-mode", "layer",
		"--tensor-split", "50,25,25",
		"--rpc", "host1:50052,host2:50052",
	}
	if len(args) != len(want) {
		t.Fatalf("buildRPCArgs() = %v (len %d), want %v (len %d)", args, len(args), want, len(want))
	}
	for i := range want {
		if args[i] != want[i] {
			t.Errorf("buildRPCArgs()[%d] = %q, want %q", i, args[i], want[i])
		}
	}
}

func TestBuildRPCArgs_NoWorkersStillIncludesCoordinatorOnlyShare(t *testing.T) {
	// A swarm with zero registered workers yet (e.g. right after "Host" is
	// pressed, before anyone joins) should still produce valid args — the
	// coordinator's own 100% share, no --rpc peers.
	args := buildRPCArgs(RPCOptions{Servers: nil, TensorSplit: []float64{100}})

	joined := strings.Join(args, " ")
	if !strings.Contains(joined, "--tensor-split 100") {
		t.Errorf("buildRPCArgs() = %q, want --tensor-split 100", joined)
	}
	if !strings.Contains(joined, "--rpc ") {
		t.Errorf("buildRPCArgs() = %q, want a (possibly empty) --rpc flag present", joined)
	}
}

func TestResolveCoordinatorBinary_NoBundledFlavorsReturnsError(t *testing.T) {
	// No binaries/ tree under this temp cwd — every flavor probe and the
	// final resolveBinary fallback should all fail to find anything,
	// resulting in a clean error rather than a panic.
	dir := t.TempDir()
	origWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("Chdir: %v", err)
	}
	t.Cleanup(func() { os.Chdir(origWd) })

	if _, err := resolveCoordinatorBinary("", "cpu"); err == nil {
		t.Error("resolveCoordinatorBinary() with no bundled binaries anywhere = nil error, want an error")
	}
}
