package e2e

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Exercise the actual HTTP -> engine -> task-local executor -> provider path.
// Wordiness alone must not multiply paid calls; explicit fan-out must retain
// project instructions and pass the writer's findings to its reviewers.
func TestTaskList_WorkerEfficiencyAndProjectRules(t *testing.T) {
	for _, parallel := range []bool{false, true} {
		name := "verbose-single-worker"
		if parallel {
			name = "explicit-parallel"
		}
		t.Run(name, func(t *testing.T) {
			h := NewHarness(t)
			root := t.TempDir()
			if err := os.MkdirAll(filepath.Join(root, ".memo"), 0755); err != nil {
				t.Fatal(err)
			}
			const rule = "TASK_EFFICIENCY_PROJECT_RULE"
			if err := os.WriteFile(filepath.Join(root, ".memo", "AGENTS.md"), []byte(rule), 0600); err != nil {
				t.Fatal(err)
			}
			h.Fake.Script = func(_ int, req FakeChatRequest) FakeChatResponse {
				if isReviewCall(req) {
					return FakeChatResponse{Text: `{"approved":true,"feedback":""}`}
				}
				return FakeChatResponse{Text: "TASK_CODER_HANDOFF: checked existing setup; verification passed"}
			}
			text := "TASK_EFFICIENCY_ITEM: " + strings.Repeat("Mevcut ayarı kontrol et. ", 20)
			if parallel {
				text = "[parallel] " + text
			}
			chat := h.NewAgentChat(root)
			list := h.CreateTaskList(chat, "efficiency", []string{text})
			h.StartTaskList(list.ID)
			final := h.WaitForTaskStatus(list.ID, 10*time.Second, "done", "failed")
			if final.Status != "done" {
				t.Fatalf("task=%+v", final)
			}
			workers, readers := 0, 0
			for _, req := range h.Fake.Requests() {
				if isReviewCall(req) || len(req.Tools) == 0 {
					continue
				}
				body := string(req.Raw)
				if !strings.Contains(body, "TASK_EFFICIENCY_ITEM") {
					continue
				}
				workers++
				if !strings.Contains(body, rule) {
					t.Errorf("worker lost project rules: %s", body)
				}
				if strings.Contains(body, "Coder output (verify this work)") {
					readers++
					if !strings.Contains(body, "TASK_CODER_HANDOFF") {
						t.Error("reader lost coder handoff")
					}
				}
			}
			want := 1
			if parallel {
				want = 4
				if readers != 3 {
					t.Errorf("readers with handoff=%d, want 3", readers)
				}
			}
			if workers != want {
				t.Errorf("paid worker calls=%d, want %d", workers, want)
			}
		})
	}
}
