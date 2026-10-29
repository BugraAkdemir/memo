// SPDX-License-Identifier: AGPL-3.0-or-later

package llama

import (
	"errors"
	"os/exec"
	"strings"
)

// ClassifyStartError names why a local engine failed to start, as one of the
// fixed codes the optional daily summary allows (docs/TELEMETRY.md). Only the
// code ever leaves the machine, never the error text, so a path or a model name
// in the message cannot travel with it. nil -> "".
func ClassifyStartError(err error) string {
	if err == nil {
		return ""
	}
	t := strings.ToLower(err.Error())
	has := func(subs ...string) bool {
		for _, s := range subs {
			if strings.Contains(t, s) {
				return true
			}
		}
		return false
	}
	switch {
	case has("permission denied"):
		return "permission"
	case has("address already in use", "only one usage of each socket"):
		return "port"
	case has("out of memory", "cannot allocate memory", "insufficient memory"):
		return "oom"
	case has("exited during startup"):
		return "crash_at_load"
	case has("failed to become ready", "deadline exceeded", "timed out"):
		return "timeout"
	case has("bulunamadı", "not found", "no such file", "cannot find the file"):
		return "missing_file"
	}
	return "unknown"
}

// ClassifyExit names how an engine process that died on its own ended:
// "oom_killed" (SIGKILL nobody sent from Memo, the kernel's out-of-memory
// killer's signature), "signal" (any other signal, e.g. a segfault),
// "exit_nonzero", else "unknown".
func ClassifyExit(err error) string {
	var ee *exec.ExitError
	if !errors.As(err, &ee) {
		return "unknown"
	}
	switch sig := exitSignal(ee); {
	case sig == sigKill:
		return "oom_killed"
	case sig != 0:
		return "signal"
	}
	if ee.ExitCode() > 0 {
		return "exit_nonzero"
	}
	return "unknown"
}

// SetExitHook registers fn to be called when the engine process dies without
// Stop() having been asked for. err is what cmd.Wait returned.
func (s *Server) SetExitHook(fn func(err error)) {
	s.mu.Lock()
	s.exitHook = fn
	s.mu.Unlock()
}
