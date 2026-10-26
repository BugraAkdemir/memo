// SPDX-License-Identifier: AGPL-3.0-or-later

package llama

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func mode(t *testing.T, p string) os.FileMode {
	t.Helper()
	fi, err := os.Stat(p)
	if err != nil {
		t.Fatal(err)
	}
	return fi.Mode().Perm()
}

func TestEnsureExecutable(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows has no execute bit")
	}
	dir := t.TempDir()

	plain := filepath.Join(dir, "plain")
	if err := os.WriteFile(plain, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !EnsureExecutable(plain) {
		t.Fatal("a 0644 file must be reported as changed")
	}
	if got := mode(t, plain); got&0o111 == 0 || got&0o600 != 0o600 {
		t.Fatalf("mode after = %o, want the execute bits added and the owner's read/write kept", got)
	}

	ready := filepath.Join(dir, "ready")
	if err := os.WriteFile(ready, []byte("x"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(ready, 0o750); err != nil {
		t.Fatal(err)
	}
	if EnsureExecutable(ready) {
		t.Fatal("an already executable file must be left alone")
	}
	if got := mode(t, ready); got != 0o750 {
		t.Fatalf("mode of an executable file changed to %o", got)
	}

	if EnsureExecutable(filepath.Join(dir, "missing")) {
		t.Fatal("a missing file is not 'changed'")
	}
	sub := filepath.Join(dir, "sub")
	if err := os.Mkdir(sub, 0o700); err != nil {
		t.Fatal(err)
	}
	if EnsureExecutable(sub) {
		t.Fatal("a directory must not be touched")
	}
}

// A file Memo does not own (a custom binary_path, something found on PATH) is
// never re-permissioned: only engine folders Memo lays out itself are.
func TestEnsureEngineExecutable_OnlyTouchesMemosOwnEngineFolders(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows has no execute bit")
	}
	root := t.TempDir()
	own := engine(t, root, "amd") // root/binaries/<os>/amd/llama-server
	if err := os.Chmod(own, 0o644); err != nil {
		t.Fatal(err)
	}
	foreign := filepath.Join(t.TempDir(), "llama-server")
	if err := os.WriteFile(foreign, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	ensureEngineExecutable(own)
	ensureEngineExecutable(foreign)

	if mode(t, own)&0o111 == 0 {
		t.Error("an engine in Memo's own folder layout was left non-executable")
	}
	if mode(t, foreign) != 0o644 {
		t.Errorf("a binary outside the layout was re-permissioned to %o", mode(t, foreign))
	}
}

// Engines pulled from R2 (rclone drops POSIX modes) or copied by an installer
// arrive as 0644. Choosing the engine runs each candidate to ask which GPUs it
// sees; a candidate that cannot be run gives no answer, which silently put a
// machine back on the old folder order. Make them runnable first.
func TestResolveBinary_AutoMakesNonExecutableEnginesRunnableBeforeAskingThem(t *testing.T) {
	requireLinux(t)
	_, bundled := withOverride(t)
	amd := probeBinary(t, bundled, "amd", noGPU, true)
	want := probeBinary(t, bundled, "cpu", vulkanGPU, true)
	for _, p := range []string{amd, want} {
		if err := os.Chmod(p, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	got, err := resolveBinary("", "auto")
	if err != nil || !samePath(t, got, want) {
		t.Fatalf("auto picked %q, %v; want the cpu engine %q", got, err, want)
	}
	if mode(t, want)&0o111 == 0 {
		t.Error("the chosen engine is still not executable")
	}
}

func TestResolveRPCServerBinary_ReturnsARunnableFile(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows has no execute bit")
	}
	_, bundled := withOverride(t)
	p := filepath.Join(bundled, "binaries", runtime.GOOS, "cpu", rpcServerBinaries()[0])
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := ResolveRPCServerBinary("cpu")
	if err != nil {
		t.Fatal(err)
	}
	if mode(t, got)&0o111 == 0 {
		t.Errorf("rpc-server %q returned non-executable (mode %o)", got, mode(t, got))
	}
}
