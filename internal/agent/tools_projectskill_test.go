// SPDX-License-Identifier: AGPL-3.0-or-later

package agent

import (
	"testing"

	"memo/internal/beta"
)

// install_project_skill belongs to project folders (Beta), and a project chat
// runs in Code Mode, so the tool must survive Code Mode's tool filter.
func TestInstallProjectSkill_IsBetaOnlyAndStaysInCodeMode(t *testing.T) {
	t.Cleanup(func() { beta.SetGate(nil) })
	r := NewRegistry()

	beta.SetGate(func() bool { return false })
	for _, d := range ForCodeMode(r.ToOpenAITools(nil)) {
		if d.Function.Name == "install_project_skill" {
			t.Fatal("install_project_skill is offered with Beta off")
		}
	}

	beta.SetGate(func() bool { return true })
	found := false
	for _, d := range ForCodeMode(r.ToOpenAITools(nil)) {
		if d.Function.Name == "install_project_skill" {
			found = true
		}
	}
	if !found {
		t.Fatal("install_project_skill must be available to a Code Mode project chat while Beta is on")
	}
	if tool, ok := r.Get("install_project_skill"); !ok || tool.DangerLevel == Safe {
		t.Fatalf("installing downloaded files must ask for permission: %+v, %v", tool, ok)
	}
}
