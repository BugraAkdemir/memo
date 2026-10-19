// SPDX-License-Identifier: AGPL-3.0-or-later

package agent

import "testing"

// A project chat runs in Code Mode by default, so install_project_skill must
// survive Code Mode's tool filter, and installing downloaded files must ask.
func TestInstallProjectSkill_StaysInCodeModeAndAsksPermission(t *testing.T) {
	r := NewRegistry()
	found := false
	for _, d := range ForCodeMode(r.ToOpenAITools(nil)) {
		if d.Function.Name == "install_project_skill" {
			found = true
		}
	}
	if !found {
		t.Fatal("install_project_skill must be available to a Code Mode project chat")
	}
	if tool, ok := r.Get("install_project_skill"); !ok || tool.DangerLevel == Safe {
		t.Fatalf("installing downloaded files must ask for permission: %+v, %v", tool, ok)
	}
}
