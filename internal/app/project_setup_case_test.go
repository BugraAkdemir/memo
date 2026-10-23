// SPDX-License-Identifier: AGPL-3.0-or-later

package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestProjectInstruction_UsesExistingRulesSpelling(t *testing.T) {
	a, chat, _, root := skillApp(t)
	canonical := filepath.Join(root, ".memo", "AGENTS.md")
	renamed := filepath.Join(root, ".memo", "agents.md")
	// Use an intermediate name so a case-insensitive filesystem also renames it.
	tmp := canonical + ".rename"
	if err := os.Rename(canonical, tmp); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(tmp, renamed); err != nil {
		t.Fatal(err)
	}
	got := a.projectInstruction(chat)
	if !strings.Contains(got, "`.memo/agents.md`") {
		t.Fatalf("wrong instruction: %s", got)
	}
	if _, err := a.SetupProject(root); err != nil {
		t.Fatal(err)
	}
	if got := a.projectInstruction(chat); !strings.Contains(got, "`.memo/agents.md`") {
		t.Fatalf("wrong instruction after repeat setup: %s", got)
	}
}
