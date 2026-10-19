// SPDX-License-Identifier: AGPL-3.0-or-later

package agent

import (
	"context"
	"strings"
	"testing"

	"memo/internal/beta"
)

var betaToolNames = []string{
	"open_app", "browser_navigate", "browser_click", "browser_type",
	"browser_scroll", "browser_screenshot", "browser_get_text", "browser_close",
	"install_project_skill",
}

func offeredNames(r *ToolRegistry) map[string]bool {
	out := map[string]bool{}
	for _, d := range r.ToOpenAITools(nil) {
		out[d.Function.Name] = true
	}
	return out
}

func TestBetaTools_AbsentFromTheRegistryWhileBetaIsOff(t *testing.T) {
	beta.SetGate(func() bool { return false })
	t.Cleanup(func() { beta.SetGate(nil) })
	r := NewRegistry()

	offered := offeredNames(r)
	for _, n := range betaToolNames {
		if offered[n] {
			t.Errorf("%s is offered to the model with Beta off", n)
		}
		if _, ok := r.Get(n); ok {
			t.Errorf("%s is found by Get with Beta off; the pipeline would run it", n)
		}
	}
	if _, err := r.Execute(context.Background(), "open_app", []byte(`{"app_name":"x"}`), "", nil); err == nil || !strings.Contains(err.Error(), "tool not found") {
		t.Errorf("Execute(open_app) with Beta off = %v, want \"tool not found\"", err)
	}
	// The gate must not touch anything else.
	if !offered["read_file"] || !offered["run_command"] {
		t.Errorf("ordinary tools vanished with Beta off: read_file=%v run_command=%v", offered["read_file"], offered["run_command"])
	}
}

func TestBetaTools_PresentWhileBetaIsOnAndFollowTheSwitch(t *testing.T) {
	on := true
	beta.SetGate(func() bool { return on })
	t.Cleanup(func() { beta.SetGate(nil) })
	r := NewRegistry()

	offered := offeredNames(r)
	for _, n := range betaToolNames {
		if !offered[n] {
			t.Errorf("%s missing with Beta on", n)
		}
		if _, ok := r.Get(n); !ok {
			t.Errorf("%s not found by Get with Beta on", n)
		}
	}

	on = false // same registry, switch flipped: no rebuild needed
	if offeredNames(r)["browser_navigate"] {
		t.Error("browser_navigate still offered after the switch went off")
	}
}

func TestBetaTools_NoGateMeansAbsent(t *testing.T) {
	beta.SetGate(nil)
	if offeredNames(NewRegistry())["open_app"] {
		t.Error("open_app offered in a process that never installed a Beta gate")
	}
}
