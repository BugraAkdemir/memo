// SPDX-License-Identifier: AGPL-3.0-or-later

package app

import "testing"

func TestGetCLIStatus_UnknownTypeNotInstalled(t *testing.T) {
	a := &App{}
	st := a.GetCLIStatus("not-a-real-cli-type").(CLIStatus)
	if st.Installed {
		t.Errorf("expected Installed=false for an unrecognized cli type")
	}
}

func TestGetCLIStatus_MissingBinaryNotInstalled(t *testing.T) {
	a := &App{}
	// claude-code-cli always maps to the "claude" binary — this just
	// verifies the not-found path doesn't error, not whether it's actually
	// installed on this machine (that varies per environment).
	st := a.GetCLIStatus("claude-code-cli").(CLIStatus)
	if st.BinaryName != "claude" {
		t.Errorf("BinaryName = %q, want claude", st.BinaryName)
	}
}
