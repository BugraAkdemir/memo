// SPDX-License-Identifier: AGPL-3.0-or-later

package app

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"memo/internal/memory"
	"memo/internal/project"
)

// codeModeProjectHarness is the real agent pipeline around a project chat
// (Code Mode defaults on for it), with a live memory store.
func codeModeProjectHarness(t *testing.T) (h *lsHarness, projectID string) {
	t.Helper()
	fp := newFakeProvider(t, func(n int, req map[string]any) map[string]any {
		return respText("fake-model", "Noted: the deployment pipeline runs on every merge to main.", 50, 10)
	})
	h = newLongSessionApp(t, fp, true)

	tmp, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	home := filepath.Join(tmp, "home")
	proj := filepath.Join(home, "work", "shop")
	if err := os.MkdirAll(proj, 0o755); err != nil {
		t.Fatal(err)
	}
	h.a.projectMgr = project.NewManager(project.Guard{Home: home}, filepath.Join(tmp, "projects.json"))
	res, err := h.a.projectManager().Scaffold(proj)
	if err != nil {
		t.Fatal(err)
	}
	h.chatID = h.sm.NewAgentChat(proj)
	if !h.a.resolveCodeMode(h.chatID) {
		t.Fatal("test setup: a project chat must default to Code Mode")
	}
	return h, res.ID
}

// finish runs what the real stream does when a turn ends (the memory save
// decision lives there) and waits for the asynchronous save worker.
func (h *lsHarness) finish(t *testing.T, userMsg, reply string) {
	t.Helper()
	ctx := withCodeMode(context.Background())
	h.a.finishStream(ctx, time.Now(), 10, "stop", reply, userMsg, h.chatID, nil)
	time.Sleep(300 * time.Millisecond) // memorySaveWorker is asynchronous
}

const deployQuestion = "how does the deployment pipeline for this service run on every merge to main"

func TestCodeMode_ProjectChat_RemembersInItsOwnPoolAndAddsNoLLMCall(t *testing.T) {
	h, id := codeModeProjectHarness(t)

	before := h.fp.callCount()
	reply, _ := h.turn(t, deployQuestion)
	if got := h.fp.callCount() - before; got != 1 {
		t.Fatalf("a Code Mode turn made %d provider calls, want exactly 1 (memory must add no LLM call)", got)
	}
	h.finish(t, deployQuestion, reply)
	if got := h.fp.callCount() - before; got != 1 {
		t.Fatalf("finishing the turn made %d provider calls in total, want 1 (no title, mood, decision or fact-extraction call in Code Mode)", got)
	}

	if n := h.a.store.Count(); n != 1 {
		t.Fatalf("store has %d rows, want 1 saved for the project", n)
	}
	ctx := context.Background()
	if got, _ := h.a.store.RetrieveContext(ctx, memory.ProjectScope(id), "deployment pipeline", 5, 0); len(got) != 1 {
		t.Fatalf("the project pool has %d rows, want 1", len(got))
	}
	if got, _ := h.a.store.RetrieveContext(ctx, memory.ProjectScope(""), "deployment pipeline", 5, 0); len(got) != 0 {
		t.Fatal("a Code Mode project turn landed in the general pool")
	}
	if facts, _ := h.a.store.GetPinnedFacts(ctx, memory.AllProjects); len(facts) != 0 {
		t.Fatalf("Code Mode learned %d personal facts about the user", len(facts))
	}

	// A NEW chat in the same project (no history of its own) gets that memory back,
	// on the user message — so only memory can be the source.
	h.chatID = h.sm.NewAgentChat(h.a.getSessionManager().GetProjectPath(h.chatID))
	_, reqs := h.turn(t, "remind me how the deployment pipeline runs on every merge")
	if len(reqs) == 0 {
		t.Fatal("no request reached the provider")
	}
	got := parseReq(t, reqs[0]).allText()
	if !strings.Contains(got, "Relevant memories") || !strings.Contains(got, "deployment pipeline for this service") {
		t.Fatalf("the project's earlier turn was not recalled in a new chat of the same project:\n%.900s", got)
	}
}

// A chat in ANOTHER project, and a plain chat's pool, must not leak into it.
func TestCodeMode_ProjectChat_NeverRecallsAnotherPool(t *testing.T) {
	h, _ := codeModeProjectHarness(t)
	ctx := context.Background()
	if err := h.a.store.SaveInteraction(ctx, memory.AllProjects, "general-secret: deployment pipeline holiday plans", "ok"); err != nil {
		t.Fatal(err)
	}
	if err := h.a.store.SaveInteraction(ctx, memory.ProjectScope("p_other"), "other-project-secret: deployment pipeline client Acme", "ok"); err != nil {
		t.Fatal(err)
	}
	_, reqs := h.turn(t, "remind me how the deployment pipeline runs")
	got := parseReq(t, reqs[0]).allText()
	for _, leak := range []string{"general-secret", "other-project-secret", "Tester"} {
		if strings.Contains(got, leak) {
			t.Fatalf("LEAK %q into a Code Mode project chat:\n%.900s", leak, got)
		}
	}
}

func TestCodeMode_ProjectChat_MemorySwitchOff_SavesAndRecallsNothing(t *testing.T) {
	h, _ := codeModeProjectHarness(t)
	h.a.cfg.Memory.MemoryEnabled = false

	reply, _ := h.turn(t, deployQuestion)
	h.finish(t, deployQuestion, reply)
	if n := h.a.store.Count(); n != 0 {
		t.Fatalf("with the Memory switch off a project turn saved %d memories", n)
	}
	res, _ := h.a.projectManager().Open(h.a.getSessionManager().GetProjectPath(h.chatID))
	if err := h.a.store.SaveInteraction(context.Background(), memory.ProjectScope(res.ID), "zebra-marker deployment pipeline notes for this service", "x"); err != nil {
		t.Fatal(err)
	}
	h.chatID = h.sm.NewAgentChat(h.a.getSessionManager().GetProjectPath(h.chatID))
	_, reqs := h.turn(t, "remind me how the deployment pipeline runs on every merge")
	if got := parseReq(t, reqs[0]).allText(); strings.Contains(got, "zebra-marker") || strings.Contains(got, "Relevant memories") {
		t.Fatalf("Memory switch off: the prompt received memory:\n%.900s", got)
	}
}

// Code Mode is only a preset for a project chat; toggling it must not change what
// the project remembers.
func TestCodeMode_ProjectMemoryDoesNotDependOnTheCodeModeToggle(t *testing.T) {
	h, id := codeModeProjectHarness(t)
	off := false
	if err := h.sm.SetCodeMode(h.chatID, &off); err != nil {
		t.Fatal(err)
	}
	if h.a.resolveCodeMode(h.chatID) {
		t.Fatal("test setup: Code Mode should be off for this chat")
	}
	reply, _ := h.turn(t, deployQuestion)
	ctx := context.Background() // not a Code Mode turn
	h.a.finishStream(ctx, time.Now(), 10, "stop", reply, deployQuestion, h.chatID, nil)
	time.Sleep(300 * time.Millisecond)
	if got, _ := h.a.store.RetrieveContext(ctx, memory.ProjectScope(id), "deployment pipeline", 5, 0); len(got) != 1 {
		t.Fatalf("project pool has %d rows with Code Mode off, want 1", len(got))
	}
	if got, _ := h.a.store.RetrieveContext(ctx, memory.ProjectScope(""), "deployment pipeline", 5, 0); len(got) != 0 {
		t.Fatal("a project turn landed in the general pool")
	}
}

func TestCodeMode_PlainChatPinnedToCodeMode_StillHasNoMemory(t *testing.T) {
	h, _ := codeModeProjectHarness(t)
	plain := h.sm.NewChat()
	on := true
	if err := h.sm.SetCodeMode(plain, &on); err != nil {
		t.Fatal(err)
	}
	h.chatID = plain

	reply, _ := h.turn(t, deployQuestion)
	h.finish(t, deployQuestion, reply)
	if n := h.a.store.Count(); n != 0 {
		t.Fatalf("a plain chat pinned to Code Mode saved %d memories; only project chats may", n)
	}
}

// A Self-Driving task step runs in a project chat in Code Mode. With the
// "task memory" setting off it must not write memory either (reading was
// already skipped);
func TestCodeMode_TaskStepWithTaskMemoryOff_WritesNothing(t *testing.T) {
	h, _ := codeModeProjectHarness(t)

	ctx := withTaskMemoryDisabled(withCodeMode(context.Background()))
	h.a.finishStream(ctx, time.Now(), 10, "stop", "Noted: the pipeline runs on every merge.", deployQuestion, h.chatID, nil)
	time.Sleep(300 * time.Millisecond)
	if n := h.a.store.Count(); n != 0 {
		t.Fatalf("a task step with task memory off saved %d memories", n)
	}
}

// A coding turn's reply can be a whole diff; the project pool keeps an excerpt.
func TestCodeMode_ProjectTurnStoresOnlyAnExcerptOfALongReply(t *testing.T) {
	h, id := codeModeProjectHarness(t)
	long := strings.Repeat("diff line of generated code\n", 400) // ~11k chars
	h.finish(t, deployQuestion, long)

	got, _ := h.a.store.RetrieveContext(context.Background(), memory.ProjectScope(id), "deployment pipeline", 5, 0)
	if len(got) != 1 {
		t.Fatalf("project pool has %d rows, want 1", len(got))
	}
	if n := len([]rune(got[0].AssistMsg)); n > projectReplyStoreChars+3 || n == 0 {
		t.Fatalf("stored reply is %d chars, want at most %d", n, projectReplyStoreChars+3)
	}
	if got[0].UserMsg != deployQuestion {
		t.Fatalf("the user's side must be stored whole, got %q", got[0].UserMsg)
	}
}
