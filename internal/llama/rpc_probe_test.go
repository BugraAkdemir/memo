package llama

import (
	"os"
	"path/filepath"
	"runtime"
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
