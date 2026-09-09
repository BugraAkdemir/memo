package llama

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"memo/internal/logx"
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

// tuningFlagCache maps a cache key (absolute binary path + "@" + mtime) to
// whether that build accepts tuningProbeFlags. Populated lazily by
// tuningFlagsSupported. The mtime is part of the key so a Settings-triggered
// engine reinstall (installer.go overwrites in place at the same path) is
// re-probed rather than served a stale verdict. Mirrors rpcCapabilityCache
// in rpc_probe.go.
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
	key := abs
	if fi, statErr := os.Stat(abs); statErr == nil {
		key = abs + "@" + fi.ModTime().UTC().Format(time.RFC3339Nano)
	}
	if cached, ok := tuningFlagCache.Load(key); ok {
		return cached.(bool)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	args := append(append([]string{}, tuningProbeFlags...), "--model", tuningProbeModel)
	out, runErr := exec.CommandContext(ctx, bin, args...).CombinedOutput()

	// A probe that never actually ran the binary to an arg-parse verdict —
	// timeout, or the process failing to start (missing file, ENOEXEC, EACCES)
	// — tells us nothing about flag support. Returning false is the safe
	// choice for THIS start (no tuning flags, but the server still comes up),
	// but caching it would silently disable --no-context-shift / --cache-reuse
	// / --flash-attn for the rest of the process even though the very next
	// start might probe cleanly (BUG-SCAN5). So log it and skip the cache.
	if ctx.Err() != nil || (runErr != nil && !ranToVerdict(runErr)) {
		logx.Printf("llama: tuning-flag probe of %s did not complete (%v); passing no tuning flags this start, will re-probe next start", filepath.Base(abs), firstErr(ctx.Err(), runErr))
		return false
	}

	supported := probeReachedModelLoad(string(out))
	if !supported {
		logx.Printf("llama: %s rejected one of %v — passing none of them (slower, but the server starts)", filepath.Base(abs), tuningProbeFlags)
	}
	tuningFlagCache.Store(key, supported)
	return supported
}

// ranToVerdict reports whether an *exec.Cmd error still means the binary ran
// and exited on its own (a non-zero exit is exactly the arg-parse-rejection
// signal probeReachedModelLoad classifies) rather than never having started.
func ranToVerdict(err error) bool {
	var exitErr *exec.ExitError
	return errors.As(err, &exitErr)
}

func firstErr(a, b error) error {
	if a != nil {
		return a
	}
	return b
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
