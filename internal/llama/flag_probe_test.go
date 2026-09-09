package llama

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
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
