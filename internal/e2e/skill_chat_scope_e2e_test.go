// SPDX-License-Identifier: AGPL-3.0-or-later

package e2e

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeScopeTestSkill writes a minimal, real skill directory whose single
// tool leaves a marker file behind when it actually runs. danger_level is
// "safe" on purpose: PermissionManager.Check short-circuits safe tools to
// allowed-without-prompt, so nothing in this test depends on the separate
// permission round trip (that already has its own E2E coverage in
// agent_permission_e2e_test.go) — if the marker file is missing, the only
// possible reason is the skill-scope gate, not an unanswered prompt.
// instructionSentinel is a string that appears nowhere but the fixture
// skill's own body, so finding it in a request proves the skill's
// instructions were injected into that chat's prompt.
const instructionSentinel = "SCOPETEST-INSTRUCTION-SENTINEL"

func writeScopeTestSkill(t *testing.T, markerPath string) string {
	t.Helper()
	dir := t.TempDir()
	skillMD := `---
name: scopetest
description: E2E fixture skill for per-chat activation
version: 1.0.0
danger_level: safe
tools:
  - name: marker
    description: Writes a marker file so the test can prove it really ran
    danger_level: safe
    command: 'echo ran > "` + markerPath + `"; echo marker-written'
    parameters:
      type: object
      properties: {}
---

# scopetest

SCOPETEST-INSTRUCTION-SENTINEL: call the marker tool when asked.
`
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(skillMD), 0644); err != nil {
		t.Fatalf("write fixture SKILL.md: %v", err)
	}
	return dir
}

// toolNamesIn extracts the function names out of one captured
// FakeChatRequest's tools array — i.e. exactly what the app actually
// offered the LLM for that turn.
func toolNamesIn(t *testing.T, req FakeChatRequest) []string {
	t.Helper()
	var names []string
	for _, raw := range req.Tools {
		var tool struct {
			Function struct {
				Name string `json:"name"`
			} `json:"function"`
		}
		if err := json.Unmarshal(raw, &tool); err != nil {
			t.Fatalf("decode tool entry %s: %v", raw, err)
		}
		names = append(names, tool.Function.Name)
	}
	return names
}

// anyRequestContains reports whether any request in reqs carries text in
// its raw body. One user turn does NOT map to exactly one provider request
// — the app makes its own auxiliary calls around a turn (chat titling and
// the like), so they interleave in FakeProvider's call log. Asserting
// against a single index silently inspects the wrong request; every
// assertion here scans that turn's whole slice instead.
func anyRequestContains(reqs []FakeChatRequest, text string) bool {
	for _, r := range reqs {
		if strings.Contains(string(r.Raw), text) {
			return true
		}
	}
	return false
}

// toolsOfferedIn unions the tool names offered across reqs, for the same
// reason anyRequestContains scans a slice.
func toolsOfferedIn(t *testing.T, reqs []FakeChatRequest) []string {
	t.Helper()
	var all []string
	for _, r := range reqs {
		for _, n := range toolNamesIn(t, r) {
			if !contains(all, n) {
				all = append(all, n)
			}
		}
	}
	return all
}

func contains(haystack []string, needle string) bool {
	for _, s := range haystack {
		if s == needle {
			return true
		}
	}
	return false
}

// TestSkill_ActivationIsScopedToOneChat is the real E2E proof for the
// per-chat skill activation work: a real skill installed over the real
// REST API, activated in ONE chat, and then exercised from two different
// chats in the same running app.
//
// It asserts both of the tool-side boundaries that design deliberately chose (see
// handoff 2026-09-19) rather than any internal state:
//
//  1. What the LLM is offered — the skill's tool must appear in the tools
//     array of the chat where the skill is active, and must be absent from
//     the other chat's.
//  2. What is allowed to execute — even when the model asks for the tool
//     anyway (this test scripts exactly that, which a real model could do
//     from stale context), the dispatch gate must refuse it in the chat
//     where the skill is off, and the tool's side effect must not happen.
//
// The prompt-side boundary is a separate test below, because it does not
// apply in a Code Mode chat at all — see that test's comment.
func TestSkill_ActivationIsScopedToOneChat(t *testing.T) {
	h := NewHarness(t)
	h.SetAgentEnabled(true)

	projectDir := t.TempDir()
	markerPath := filepath.Join(projectDir, "marker.txt")
	skillName := h.InstallSkill(writeScopeTestSkill(t, markerPath))
	if skillName != "scopetest" {
		t.Fatalf("installed skill name = %q, want scopetest", skillName)
	}
	const toolName = "skill_scopetest_marker"

	activeChat := h.NewAgentChat(projectDir)
	otherChat := h.NewAgentChat(projectDir)

	// A brand-new chat must start with NO skills active — that is the whole
	// point of dropping the old global, persisted active-skill list.
	if got := h.GetChatActiveSkills(activeChat); len(got) != 0 {
		t.Fatalf("a fresh chat already has active skills %v, want none", got)
	}

	h.SetChatActiveSkills(activeChat, []string{skillName})
	if got := h.GetChatActiveSkills(activeChat); !contains(got, skillName) {
		t.Fatalf("after activating, chat active skills = %v, want to contain %q", got, skillName)
	}
	// Activating in one chat must not leak into any other chat.
	if got := h.GetChatActiveSkills(otherChat); len(got) != 0 {
		t.Fatalf("activating a skill in one chat leaked into another chat: %v", got)
	}

	// Both turns are scripted to ask for the skill tool on their first call.
	h.Fake.Script = func(callNum int, req FakeChatRequest) FakeChatResponse {
		if len(req.Tools) > 0 && !strings.Contains(string(req.Raw), `"tool_calls"`) {
			return FakeChatResponse{ToolCalls: []FakeToolCall{{
				ID:        "call_1",
				Name:      toolName,
				Arguments: `{}`,
			}}}
		}
		return FakeChatResponse{Text: "bitti"}
	}

	// --- Chat A: skill active ---
	h.SendMessageStream(activeChat, "marker aracini calistir")

	reqsAfterA := h.Fake.Requests()
	if len(reqsAfterA) == 0 {
		t.Fatal("the active chat's turn never reached the provider")
	}
	offeredA := toolsOfferedIn(t, reqsAfterA)
	if !contains(offeredA, toolName) {
		t.Fatalf("chat with the skill ACTIVE was not offered %q; offered tools: %v", toolName, offeredA)
	}
	if _, err := os.Stat(markerPath); err != nil {
		t.Fatalf("the skill tool never actually ran in the chat where its skill is active: %v", err)
	}

	// Remove the marker so the second chat's run is proven independently.
	if err := os.Remove(markerPath); err != nil {
		t.Fatalf("reset marker: %v", err)
	}

	// --- Chat B: same skill, never activated here ---
	before := len(h.Fake.Requests())
	h.SendMessageStream(otherChat, "marker aracini calistir")

	reqsAfterB := h.Fake.Requests()
	if len(reqsAfterB) <= before {
		t.Fatal("the second chat's turn never reached the provider")
	}
	offeredB := toolsOfferedIn(t, reqsAfterB[before:])
	if contains(offeredB, toolName) {
		t.Fatalf("chat with the skill INACTIVE was still offered %q; offered tools: %v", toolName, offeredB)
	}
	// Non-skill tools must still be there — the gate filters skill-owned
	// tools, it does not empty the whole tool list.
	if len(offeredB) == 0 {
		t.Fatal("the inactive chat was offered no tools at all — the gate filtered more than skill-owned tools")
	}
	if _, err := os.Stat(markerPath); err == nil {
		t.Fatal("the skill tool executed in a chat where its skill is NOT active — the dispatch gate did not hold")
	} else if !os.IsNotExist(err) {
		t.Fatalf("unexpected error checking the marker file: %v", err)
	}
}

// TestSkill_InstructionsReachOnlyTheActivatedChat covers the other half of
// per-chat skill activation: the skill's own instruction text in the system
// prompt. This is the half the old design got most visibly wrong — one
// global, persisted active-skill list meant a skill turned on anywhere
// injected its full instructions into every chat forever (the live
// incident in buildActiveSkillPrompt's doc comment: 5 imported skills
// blowing a 4096-ctx model's whole budget on a bare "selam").
//
// Uses plain chats (NewChat), NOT agent/project chats, and that distinction
// is load-bearing rather than incidental: resolveCodeMode defaults a chat
// with a ProjectPath into Code Mode, and buildSystemPrompt's `case code:`
// branch replaces the entire persona/memory/skill stack with one compact
// coding directive — so an agent chat never receives skill instructions
// whether or not the skill is active there. That is pre-existing behavior,
// not something per-chat activation changed; asserting the prompt boundary
// in an agent chat would just be asserting Code Mode's own emptiness.
func TestSkill_InstructionsReachOnlyTheActivatedChat(t *testing.T) {
	h := NewHarness(t)

	markerPath := filepath.Join(t.TempDir(), "unused-marker.txt")
	skillName := h.InstallSkill(writeScopeTestSkill(t, markerPath))

	activeChat := h.NewChat()
	otherChat := h.NewChat()
	h.SetChatActiveSkills(activeChat, []string{skillName})

	h.SendMessageStream(activeChat, "selam")
	reqs := h.Fake.Requests()
	if len(reqs) == 0 {
		t.Fatal("the active chat's turn never reached the provider")
	}
	if !anyRequestContains(reqs, instructionSentinel) {
		t.Fatalf("the chat with the skill ACTIVE never received the skill's instructions (%q) in its prompt", instructionSentinel)
	}

	before := len(reqs)
	h.SendMessageStream(otherChat, "selam")
	reqs = h.Fake.Requests()
	if len(reqs) <= before {
		t.Fatal("the second chat's turn never reached the provider")
	}
	if anyRequestContains(reqs[before:], instructionSentinel) {
		t.Fatalf("the skill's instructions (%q) leaked into a chat where the skill was never activated", instructionSentinel)
	}
}
