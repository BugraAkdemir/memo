package llama

import (
	"context"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// flashAttnProbeModel is a deliberately-nonexistent model path handed to the
// probe below — an flag-accepting build proceeds past argument parsing and
// then fails trying to load it, which is exactly the signal we key on.
const flashAttnProbeModel = "__memo_fa_probe_nonexistent__.gguf"

// flashAttnCapabilityCache maps an absolute llama-server binary path to
// whether it was found to accept --flash-attn. Populated lazily by
// flashAttnSupported, kept for the process's lifetime — the bundled binary a
// path points at doesn't change while Memo is running. Mirrors
// rpcCapabilityCache in rpc_probe.go.
var flashAttnCapabilityCache sync.Map // map[string]bool

// flashAttnSupported reports whether bin's llama-server build accepts the
// bare --flash-attn flag. This is probed rather than assumed because the
// flag's spelling and acceptance have shifted across llama.cpp releases
// (bare boolean vs. --flash-attn on|off|auto with a required value) and the
// bundled binary tracks upstream's latest release (see installer.go, which
// pulls releases/latest). Passing an unrecognized or now-value-requiring
// flag makes llama-server exit at argument-parsing time, so an unconditional
// --flash-attn would be a hard "model won't start" regression on some builds.
//
// Classification is delegated to flashAttnProbeAccepted so it can be unit
// tested without a real binary. Result cached per absolute binary path.
func flashAttnSupported(bin string) bool {
	abs, err := filepath.Abs(bin)
	if err != nil {
		abs = bin
	}
	if cached, ok := flashAttnCapabilityCache.Load(abs); ok {
		return cached.(bool)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin,
		"--flash-attn",
		"--model", flashAttnProbeModel,
	)
	out, _ := cmd.CombinedOutput()
	supported := flashAttnProbeAccepted(string(out))

	flashAttnCapabilityCache.Store(abs, supported)
	return supported
}

// flashAttnProbeAccepted decides, from a probe run's combined output, whether
// llama-server got past argument parsing with --flash-attn set — i.e. it
// reached model loading and failed there on the nonexistent probe path.
//
// This is robust to both rejection modes: an unknown flag ("invalid
// argument: --flash-attn") and a flag that now requires a value ("expected
// value", "requires an argument") both abort before model loading, so none
// of the model-load markers appear and the result is a safe false — Memo
// then simply doesn't pass the flag, i.e. today's behavior, no regression.
func flashAttnProbeAccepted(output string) bool {
	s := strings.ToLower(output)
	// Any complaint that names the flag itself is a definitive rejection,
	// regardless of what else the output contains.
	for _, reject := range []string{
		"invalid argument: --flash-attn",
		"invalid argument: -fa",
		"unrecognized argument: --flash-attn",
		"error: invalid argument",
	} {
		if strings.Contains(s, reject) {
			return false
		}
	}
	// Accepted => it went on to try loading flashAttnProbeModel and failed.
	for _, reached := range []string{
		strings.ToLower(flashAttnProbeModel),
		"failed to load model",
		"error loading model",
		"load_model",
		"loading model",
		"no such file",
		"cannot open",
		"unable to load model",
	} {
		if strings.Contains(s, reached) {
			return true
		}
	}
	// Unknown wording — assume unsupported so we never pass a flag that
	// might abort startup.
	return false
}
