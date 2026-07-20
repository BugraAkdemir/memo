package llama

import (
	"context"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"memo/internal/logx"
)

// buildRPCArgs returns the extra llama-server CLI args needed to run as a
// swarm coordinator. Always forces --split-mode layer — never "row": row
// mode needs much tighter per-layer synchronization than layer mode, fine
// over NVLink/PCIe but far too slow over a normal home network (see
// PLAN_memo_swarm.md's design decisions) — this is a hard invariant, not a
// caller-configurable choice, so it's baked in here rather than threaded
// through RPCOptions.
func buildRPCArgs(opts RPCOptions) []string {
	splits := make([]string, len(opts.TensorSplit))
	for i, v := range opts.TensorSplit {
		splits[i] = strconv.FormatFloat(v, 'f', -1, 64)
	}
	return []string{
		"--split-mode", "layer",
		"--tensor-split", strings.Join(splits, ","),
		"--rpc", strings.Join(opts.Servers, ","),
	}
}

// rpcCapabilityCache maps an absolute llama-server binary path to whether it
// was found to accept --rpc. Populated lazily by probeRPCSupport, kept for
// the process's lifetime — the bundled binary a path points at doesn't
// change while Memo is running.
var rpcCapabilityCache sync.Map // map[string]bool

// probeRPCSupport reports whether bin's llama-server build accepts --rpc at
// all. Verified empirically (2026-07-20): some bundled release flavors
// (linux/cpu) aren't compiled with RPC support and fail fast at
// arg-parsing time ("error: invalid argument: --rpc"), while RPC-capable
// builds (linux/nvidia, linux/amd) accept the flag and fail later instead,
// on the deliberately-nonexistent model path this probe passes — either way
// the process exits almost immediately, so this is cheap to run. Result is
// cached per absolute binary path (see rpcCapabilityCache).
func probeRPCSupport(bin string) bool {
	abs, err := filepath.Abs(bin)
	if err != nil {
		abs = bin
	}
	if cached, ok := rpcCapabilityCache.Load(abs); ok {
		return cached.(bool)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin,
		"--model", "__memo_rpc_probe_nonexistent__.gguf",
		"--rpc", "127.0.0.1:1",
	)
	out, _ := cmd.CombinedOutput()
	supported := !strings.Contains(string(out), "invalid argument: --rpc")

	rpcCapabilityCache.Store(abs, supported)
	return supported
}

// resolveCoordinatorBinary finds an llama-server binary capable of --rpc,
// for the swarm coordinator ("Host") role specifically. Some bundled
// flavors (verified: linux/cpu) aren't built with RPC support even though
// the local machine itself may be CPU-only — the GPU-flavored binaries
// (nvidia/amd) run fine on a GPU-less machine too, they simply don't use a
// GPU. So for this role only, flavor preference is nvidia > amd > cpu,
// deliberately independent of the machine's own detected hardware or the
// caller's requested mode, and each candidate is actually probed rather
// than assumed capable (a future llama.cpp release could change which
// flavors ship with RPC support). Falls back to the normal resolveBinary
// result (with a logged warning) if nothing probes capable — callers should
// surface that as a clear "swarm not supported with this build" error
// rather than silently trying to launch with --rpc anyway.
func resolveCoordinatorBinary(configured, mode string) (string, error) {
	for _, flavor := range []string{"nvidia", "amd", "cpu"} {
		bin, err := resolveBinary(configured, flavor)
		if err != nil {
			continue
		}
		if probeRPCSupport(bin) {
			return bin, nil
		}
	}

	bin, err := resolveBinary(configured, mode)
	if err != nil {
		return "", err
	}
	logx.Printf("swarm: no RPC-capable llama-server flavor found for the coordinator role, falling back to %s (may not support --rpc)", bin)
	return bin, nil
}
