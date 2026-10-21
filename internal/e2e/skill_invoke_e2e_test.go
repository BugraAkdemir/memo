// SPDX-License-Identifier: AGPL-3.0-or-later

package e2e

import (
	"path/filepath"
	"strings"
	"testing"
)

// TestSkill_CallingASkillSwitchesItOnForThatChatOnly is the real-wiring proof
// of "a skill is used by calling it": no toggle anywhere, the message itself
// ("... /scopetest kullanarak çöz") switches the skill on for THAT chat — its
// instructions are in the prompt of the very turn that called it, the stream
// says which skill is being used, the saved message keeps that line, and a
// different chat never sees any of it.
func TestSkill_CallingASkillSwitchesItOnForThatChatOnly(t *testing.T) {
	h := NewHarness(t)
	h.SetWebSearchEnabled(false)

	skillName := h.InstallSkill(writeScopeTestSkill(t, filepath.Join(t.TempDir(), "marker.txt")))
	callerChat := h.NewChat()
	otherChat := h.NewChat()

	events := h.SendMessageStream(callerChat, "login sorununu /"+skillName+" kullanarak çöz")

	if got := h.GetChatActiveSkills(callerChat); len(got) != 1 || got[0] != skillName {
		t.Fatalf("active skills of the calling chat = %v, want [%s]", got, skillName)
	}
	if !anyRequestContains(h.Fake.Requests(), instructionSentinel) {
		t.Fatal("the skill was called but its instructions never reached the model in that turn")
	}

	sawNotice := false
	for _, e := range events {
		if e.FinishReason == "agent_event" && strings.Contains(e.Content, `"skill_used"`) && strings.Contains(e.Content, skillName) {
			sawNotice = true
		}
	}
	if !sawNotice {
		t.Errorf("the stream never said which skill was being used: %+v", events)
	}

	var saved []struct {
		Role        string        `json:"role"`
		AgentEvents []interface{} `json:"agent_events"`
	}
	decodeInto(t, h.getJSON("/api/messages?chat_id="+callerChat), &saved)
	kept := false
	for _, m := range saved {
		if m.Role != "assistant" {
			continue
		}
		for _, ev := range m.AgentEvents {
			if mm, ok := ev.(map[string]interface{}); ok && mm["type"] == "skill_used" && mm["tool"] == skillName {
				kept = true
			}
		}
	}
	if !kept {
		t.Errorf("the saved reply lost its 'using skill' line: %+v", saved)
	}

	// Another chat, and a brand new one, start without it.
	before := len(h.Fake.Requests())
	h.SendMessageStream(otherChat, "selam")
	if anyRequestContains(h.Fake.Requests()[before:], instructionSentinel) {
		t.Error("the skill's instructions leaked into a chat that never called it")
	}
	if got := h.GetChatActiveSkills(otherChat); len(got) != 0 {
		t.Errorf("another chat has active skills %v", got)
	}
	if got := h.GetChatActiveSkills(h.NewChat()); len(got) != 0 {
		t.Errorf("a new chat starts with active skills %v", got)
	}
}
