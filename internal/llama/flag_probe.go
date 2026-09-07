package llama

import (
	"context"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// tuningProbeModel is a deliberately-nonexistent model path handed to the
// probe below — a build that accepts every tuning flag proceeds past
// argument parsing and then fails trying to load it, which is exactly the
// signal we key on.
const tuningProbeModel = "__memo_tuning_probe_nonexistent__.gguf"

// tuningProbeFlags are the perf/behaviour flags Memo wants to pass to a chat
// llama-server but that an older or stripped build may not recognize. They
// are probed together in one exec: if the build rejects ANY of them it bails
// at arg-parsing before model load, and Memo then passes NONE of them —
// slower, but the server still starts. See tuningFlagsSupported.
var tuningProbeFlags = []string{
	"--flash-attn",
	"--no-context-shift",
	"--cache-reuse", "256",
}

// tuningFlagCache maps an absolute llama-server binary path to whether it
// accepts tuningProbeFlags. Populated lazily by tuningFlagsSupported, kept
// for the process's lifetime — the bundled binary a path points at doesn't
// change while Memo is running. Mirrors rpcCapabilityCache in rpc_probe.go.
var tuningFlagCache sync.Map // map[string]bool

// tuningFlagsSupported reports whether bin's llama-server build accepts all
// of tuningProbeFlags. These are probed rather than assumed because their
// spelling/acceptance has shifted across llama.cpp releases (e.g.
// --flash-attn went from a bare boolean to on|off|auto with a value;
// --cache-reuse only landed mid-2024) and Memo can be pointed at an
// arbitrary binary via llama.binary_path or PATH discovery. Passing an
// unrecognized flag makes llama-server exit at argument-parsing time, so an
// unconditional flag would be a hard "model won't start" regression on some
// builds.
//
// Classification is delegated to probeReachedModelLoad so it can be unit
// tested without a real binary. Result cached per absolute binary path.
func tuningFlagsSupported(bin string) bool {
	abs, err := filepath.Abs(bin)
	if err != nil {
		abs = bin
	}
	if cached, ok := tuningFlagCache.Load(abs); ok {
		return cached.(bool)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	args := append(append([]string{}, tuningProbeFlags...), "--model", tuningProbeModel)
	out, _ := exec.CommandContext(ctx, bin, args...).CombinedOutput()
	supported := probeReachedModelLoad(string(out))

	tuningFlagCache.Store(abs, supported)
	return supported
}

// probeReachedModelLoad decides, from a probe run's combined output, whether
// llama-server got past argument parsing with the tuning flags set — i.e. it
// reached model loading and failed there on the nonexistent probe path.
//
// This is robust to every rejection mode: an unknown flag ("invalid
// argument: --cache-reuse"), a flag that now requires a value ("expected
// value"), etc. all abort before model loading, so none of the model-load
// markers appear and the result is a safe false — Memo then passes none of
// the tuning flags, i.e. the pre-tuning behavior, no regression.
func probeReachedModelLoad(output string) bool {
	s := strings.ToLower(output)
	// Any complaint that names one of the flags, or a generic arg-parse
	// error, is a definitive rejection regardless of what else appears.
	for _, reject := range []string{
		"invalid argument",
		"unrecognized argument",
		"unknown argument",
		"expected value",
		"requires an argument",
	} {
		if strings.Contains(s, reject) {
			return false
		}
	}
	// Accepted => it went on to try loading tuningProbeModel and failed.
	for _, reached := range []string{
		strings.ToLower(tuningProbeModel),
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
