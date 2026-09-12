// SPDX-License-Identifier: AGPL-3.0-or-later

package skills

import "testing"

// TestMemoSystemFS_ContainsSkillMd guards the go:embed contract:
// skill.MaterializeEmbedded (internal/skill) reads this exact path at
// startup to write the built-in skill to disk. A rename/move of
// memo-system/SKILL.md would fail the build outright (go:embed requires
// the path to exist), but a change to its *content* — e.g. accidentally
// emptying the file — would not, and would only surface later as a
// built-in skill with no instructions.
func TestMemoSystemFS_ContainsSkillMd(t *testing.T) {
	data, err := MemoSystemFS.ReadFile("memo-system/SKILL.md")
	if err != nil {
		t.Fatalf("ReadFile(memo-system/SKILL.md): %v", err)
	}
	if len(data) == 0 {
		t.Error("memo-system/SKILL.md is embedded but empty")
	}
}
