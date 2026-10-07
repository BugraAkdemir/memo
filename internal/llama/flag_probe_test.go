package llama

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestProbeReachedModelLoad(t *testing.T) {
	tests := []struct {
		name   string
		output string
		want   bool
	}{
		{
			name:   "unknown flag — old or stripped build",
			output: "error: invalid argument: --cache-reuse\nusage: llama-server [options]",
			want:   false,
		},
		{
			name:   "flash-attn now requires a value, aborts before model load",
			output: "error while handling argument \"--flash-attn\": expected value\n",
			want:   false,
		},
		{
			name:   "no-context-shift unrecognized",
			output: "main: error: unrecognized argument: --no-context-shift\n",
			want:   false,
		},
		{
			name:   "accepted — reached model load and failed on the probe path",
			output: "llama_model_load: error loading model: failed to open __memo_tuning_probe_nonexistent__.gguf\n",
			want:   true,
		},
		{
			name:   "accepted — generic load failure wording",
			output: "common_init_from_params: failed to load model 'x'\nmain: error: unable to load model\n",
			want:   true,
		},
		{
			name:   "accepted — no such file",
			output: "gguf_init_from_file: failed to open '__memo_tuning_probe_nonexistent__.gguf': No such file or directory\n",
			want:   true,
		},
		{
			name:   "unrecognised wording — assume unsupported, never risk a bad flag",
			output: "something totally different happened\n",
			want:   false,
		},
		{
			name:   "reject wins even if a load marker also appears",
			output: "loading model...\nerror: invalid argument: --flash-attn\n",
			want:   false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := probeReachedModelLoad(tt.output); got != tt.want {
				t.Errorf("probeReachedModelLoad(%q) = %v, want %v", tt.output, got, tt.want)
			}
		})
	}
}

// TestTuningFlagsSupported_TransientFailureNotCached guards BUG-SCAN5: a
// probe that never ran the binary to an arg-parse verdict (here: the binary
// does not exist) must return false for this start but must NOT be cached —
// otherwise one bad probe disables the tuning flags for the whole process.
func TestTuningFlagsSupported_TransientFailureNotCached(t *testing.T) {
	bin := filepath.Join(t.TempDir(), "does-not-exist-llama-server")
	if tuningFlagsSupported(bin) {
		t.Fatalf("missing binary should probe as unsupported")
	}
	abs, _ := filepath.Abs(bin)
	if _, ok := tuningFlagCache.Load(abs); ok {
		t.Fatalf("a failed-to-start probe was cached; it must be re-probed next start")
	}
}

// TestTuningFlagsSupported_CachesRealVerdict: a binary that actually runs and
// reaches (a simulated) model load is classified and cached, and the cache
// key carries the mtime so a rebuild at the same path re-probes.
func TestTuningFlagsSupported_CachesRealVerdict(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell-script stub is POSIX-only")
	}
	dir := t.TempDir()
	bin := filepath.Join(dir, "fake-llama-server")
	script := "#!/bin/sh\necho 'llama_model_load: error loading model: failed to open __memo_tuning_probe_nonexistent__.gguf' >&2\nexit 1\n"
	if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	if !tuningFlagsSupported(bin) {
		t.Fatalf("stub that reaches model load should probe as supported")
	}
	abs, _ := filepath.Abs(bin)
	fi, _ := os.Stat(abs)
	if _, ok := tuningFlagCache.Load(abs + "@" + fi.ModTime().UTC().Format(time.RFC3339Nano)); !ok {
		t.Fatalf("a real verdict should be cached under the path@mtime key")
	}
}

func TestRanToVerdict(t *testing.T) {
	// A binary that exits non-zero produces an *exec.ExitError — it ran.
	err := exec.Command("sh", "-c", "exit 3").Run()
	if !ranToVerdict(err) {
		t.Errorf("non-zero exit should count as having run to a verdict")
	}
	// A binary that cannot start does not.
	err = exec.Command(filepath.Join(t.TempDir(), "nope")).Run()
	if ranToVerdict(err) {
		t.Errorf("failure to start should not count as a verdict")
	}
}

// stubServer writes a POSIX shell script that behaves like a llama-server whose
// --flash-attn is spelled the given way, so flashAttnArgs can be tested without a
// real binary:
//
//	"bare"  — old builds: `--flash-attn` is a boolean; a value after it is a stray
//	          positional ("invalid argument").
//	"value" — new builds (b11456): `--flash-attn` REQUIRES on|off|auto.
//	"none"  — a build that has never heard of the flag.
func stubServer(t *testing.T, kind string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("shell-script stub is POSIX-only")
	}
	body := map[string]string{
		"bare": `
prev=""
for a in "$@"; do
  if [ "$prev" = "--flash-attn" ] && [ "$a" = "on" ]; then echo "error: invalid argument: on" >&2; exit 1; fi
  prev="$a"
done`,
		"value": `
prev=""
for a in "$@"; do
  if [ "$prev" = "--flash-attn" ] && [ "${a#--}" != "$a" ]; then echo 'error while handling argument "--flash-attn": expected value for argument' >&2; exit 1; fi
  prev="$a"
done
if [ "$prev" = "--flash-attn" ]; then echo 'error while handling argument "--flash-attn": expected value for argument' >&2; exit 1; fi`,
		"none": `
for a in "$@"; do
  if [ "$a" = "--flash-attn" ]; then echo "error: invalid argument: --flash-attn" >&2; exit 1; fi
done`,
	}[kind]
	bin := filepath.Join(t.TempDir(), "llama-server-"+kind)
	script := "#!/bin/sh\n" + body + "\necho 'llama_model_load: error loading model: failed to open __memo_tuning_probe_nonexistent__.gguf' >&2\nexit 1\n"
	if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return bin
}

// The two spellings are mutually exclusive between builds, so the one to pass has
// to be probed: guessing the new one breaks every older engine, guessing the old
// one broke (and, through the shared probe, silently disabled all tuning on)
// current ones.
func TestFlashAttnArgs_PicksTheSpellingTheBuildAccepts(t *testing.T) {
	cases := []struct {
		kind string
		want []string
	}{
		{"value", []string{"--flash-attn", "on"}},
		{"bare", []string{"--flash-attn"}},
		{"none", nil},
	}
	for _, c := range cases {
		got := flashAttnArgs(stubServer(t, c.kind))
		if strings.Join(got, " ") != strings.Join(c.want, " ") {
			t.Errorf("%s build: flashAttnArgs = %v, want %v", c.kind, got, c.want)
		}
	}
}

// Core tuning flags must not ride on flash-attention's verdict: a build that
// rejects only the bare --flash-attn still gets --no-context-shift and
// --cache-reuse.
func TestTuningFlags_AreIndependentOfFlashAttnSpelling(t *testing.T) {
	bin := stubServer(t, "value") // rejects bare --flash-attn, accepts the rest
	if !tuningFlagsSupported(bin) {
		t.Error("--no-context-shift / --cache-reuse were disabled because of --flash-attn's spelling")
	}
	for _, f := range tuningProbeFlags {
		if f == "--flash-attn" {
			t.Error("--flash-attn must not be part of the shared tuning probe")
		}
	}
}

func TestFlashAttnArgs_ABinaryThatDoesNotStartIsNotCached(t *testing.T) {
	bin := filepath.Join(t.TempDir(), "missing-llama-server")
	if got := flashAttnArgs(bin); got != nil {
		t.Errorf("got %v for a missing binary, want nil", got)
	}
	abs, _ := filepath.Abs(bin)
	if _, ok := flashAttnCache.Load(abs); ok {
		t.Error("a failed-to-start probe was cached; it must be re-probed next start")
	}
}

// llama.cpp renamed rpc-server to ggml-rpc-server. A bundled tree can hold either
// (each platform folder is refreshed from a release on its own schedule), so the
// swarm's worker lookup must find both, newest name first.
func TestResolveRPCServerBinary_FindsEitherName(t *testing.T) {
	for _, tc := range []struct{ name, create, want string }{
		{"only the new name", "ggml-rpc-server", "ggml-rpc-server"},
		{"only the old name", "rpc-server", "rpc-server"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			dir := filepath.Join(root, "binaries", runtime.GOOS, "cpu")
			if err := os.MkdirAll(dir, 0o755); err != nil {
				t.Fatal(err)
			}
			file := tc.create
			if runtime.GOOS == "windows" {
				file += ".exe"
			}
			if err := os.WriteFile(filepath.Join(dir, file), []byte("x"), 0o755); err != nil {
				t.Fatal(err)
			}
			t.Chdir(root)
			got, err := ResolveRPCServerBinary("cpu")
			if err != nil {
				t.Fatalf("ResolveRPCServerBinary: %v", err)
			}
			if filepath.Base(got) != file {
				t.Errorf("got %s, want %s", filepath.Base(got), file)
			}
		})
	}

	// Both present: the new name wins.
	root := t.TempDir()
	dir := filepath.Join(root, "binaries", runtime.GOOS, "cpu")
	os.MkdirAll(dir, 0o755)
	for _, n := range rpcServerBinaries() {
		os.WriteFile(filepath.Join(dir, n), []byte("x"), 0o755)
	}
	t.Chdir(root)
	got, err := ResolveRPCServerBinary("cpu")
	if err != nil || filepath.Base(got) != rpcServerBinaries()[0] {
		t.Errorf("with both present got %v (%v), want %s", got, err, rpcServerBinaries()[0])
	}
}
