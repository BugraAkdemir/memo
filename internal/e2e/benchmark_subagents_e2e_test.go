// SPDX-License-Identifier: AGPL-3.0-or-later

package e2e

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"memo/internal/subagents"
)

// TestBenchmark_SubagentsFakeProject runs a complete end-to-end benchmark
// scenario for normal-chat subagents operating on a realistic fake project.
//
// Scenario phases:
// 1. Initialize a real Git project directory with initial code and tests.
// 2. Main agent roots at the project in agent mode.
// 3. Main agent spawns two isolated subagents (child-1 for math, child-2 for string utils).
// 4. Verify parallel execution, separate memo-* worktrees and branches.
// 5. Children write code and run verification tests in their respective worktrees.
// 6. Main branch remains clean and untouched while children are working.
// 7. Follow-up instruction (send_agent) sent to child-1 to add additional functionality.
// 8. Completed results arrive in main agent's durable FIFO mailbox.
// 9. Main agent reviews, merges (merge_agent) child-1 then child-2 serially.
// 10. Verify main branch contains both changes, and all temporary branches/worktrees are cleaned up.
func TestBenchmark_SubagentsFakeProject(t *testing.T) {
	// Step 1: Create a real Git repo for the fake project
	projectDir := t.TempDir()
	runGit(t, projectDir, "init", "-b", "main")
	runGit(t, projectDir, "config", "user.name", "Benchmark Engineer")
	runGit(t, projectDir, "config", "user.email", "benchmark@google.test")
	runGit(t, projectDir, "config", "commit.gpgsign", "false")

	// Base project files
	initialPy := "def add(a, b):\n    return a + b\n"
	if err := os.WriteFile(filepath.Join(projectDir, "math_utils.py"), []byte(initialPy), 0644); err != nil {
		t.Fatal(err)
	}
	testPy := "from math_utils import add\n\ndef test_add():\n    assert add(2, 3) == 5\n"
	if err := os.WriteFile(filepath.Join(projectDir, "test_math.py"), []byte(testPy), 0644); err != nil {
		t.Fatal(err)
	}
	runGit(t, projectDir, "add", ".")
	runGit(t, projectDir, "commit", "-m", "chore: initial project commit")

	// Step 2: Boot Harness and Agent Chat
	h := NewHarness(t)
	h.SetAgentEnabled(true)
	h.SetAutoPermission(true)
	chat := h.NewAgentChat(projectDir)

	var child1Spawned, child2Spawned atomic.Bool
	childTasks := make(chan FakeChatRequest, 10)
	mainReviews := make(chan string, 10)

	h.Fake.Script = func(_ int, req FakeChatRequest) FakeChatResponse {
		last := lastSubAgentUser(req)

		// Child execution
		if childRequest(req) {
			childTasks <- req
			if strings.Contains(last, "TASK_MULTIPLY_FOLLOWUP") {
				return FakeChatResponse{Text: "SUBAGENT_1_FOLLOWUP_DONE: Added multiply and docstring"}
			}
			if strings.Contains(last, "TASK_MULTIPLY") {
				return FakeChatResponse{Text: "SUBAGENT_1_DONE: Implemented multiply in math_utils.py"}
			}
			if strings.Contains(last, "TASK_REVERSE") {
				return FakeChatResponse{Text: "SUBAGENT_2_DONE: Implemented reverse_string in string_utils.py"}
			}
			return FakeChatResponse{Text: "CHILD_GENERIC_DONE"}
		}

		// Main agent handling returned subagent result
		if strings.Contains(last, "<memo-subagent-result>") && len(req.Tools) > 0 {
			mainReviews <- last
			return FakeChatResponse{Text: "MAIN: Reviewed subagent completion."}
		}

		// Initial user instruction to delegate
		if strings.Contains(last, "DELEGATE_BENCHMARK_TASKS") {
			if child1Spawned.CompareAndSwap(false, true) {
				return FakeChatResponse{
					ToolCalls: []FakeToolCall{{
						ID:   "call-spawn-1",
						Name: "spawn_agent",
						Arguments: `{"prompt":"TASK_MULTIPLY: Add multiply(a, b) function to math_utils.py and test in test_math.py","title":"Math utils multiply","write":true,"isolate":true}`,
					}},
				}
			}
		}

		return FakeChatResponse{Text: "MAIN: Ready for instructions."}
	}

	// Step 3: Trigger main agent to spawn child 1
	events := h.SendMessageStream(chat, "DELEGATE_BENCHMARK_TASKS please delegate the work.")
	for _, ev := range events {
		if ev.Error != "" {
			t.Fatalf("stream error: %s", ev.Error)
		}
	}

	if !child1Spawned.Load() {
		t.Fatal("main agent did not invoke spawn_agent for child 1")
	}

	// Wait for child 1 to be created and verify isolation
	job1 := awaitSubAgentJob(t, h, chat, func(j subagents.Job) bool {
		return strings.Contains(j.Prompt, "TASK_MULTIPLY")
	})
	if !strings.HasPrefix(job1.Branch, "memo-") {
		t.Fatalf("child 1 expected memo- branch, got %q", job1.Branch)
	}
	if job1.WorkDir == projectDir || !strings.Contains(job1.WorkDir, "subagents/worktrees") {
		t.Fatalf("child 1 expected isolated workdir, got %q", job1.WorkDir)
	}

	// Step 4: Spawn child 2 directly to test parallel isolated execution
	args2, _ := json.Marshal(map[string]any{
		"prompt":  "TASK_REVERSE: Create string_utils.py with reverse_string(s) function",
		"title":   "String utils reverse",
		"write":   true,
		"isolate": true,
	})
	respCreated, err := h.App.ChatSubAgentAction(context.Background(), chat, "spawn", args2)
	if err != nil {
		t.Fatalf("spawn child 2 error: %v", err)
	}
	var job2 subagents.Job
	if err = json.Unmarshal([]byte(respCreated), &job2); err != nil {
		t.Fatal(err)
	}
	child2Spawned.Store(true)

	if !strings.HasPrefix(job2.Branch, "memo-") || job2.Branch == job1.Branch {
		t.Fatalf("child 2 branch clash: job1=%s job2=%s", job1.Branch, job2.Branch)
	}
	if job2.WorkDir == job1.WorkDir || job2.WorkDir == projectDir {
		t.Fatalf("child 2 workdir clash: job1=%s job2=%s", job1.WorkDir, job2.WorkDir)
	}

	// Step 5: Simulate children writing code into their isolated worktrees
	// Child 1 edits math_utils.py in its workdir
	child1Math := initialPy + "\ndef multiply(a, b):\n    return a * b\n"
	if err := os.WriteFile(filepath.Join(job1.WorkDir, "math_utils.py"), []byte(child1Math), 0644); err != nil {
		t.Fatal(err)
	}
	runGit(t, job1.WorkDir, "add", "math_utils.py")
	runGit(t, job1.WorkDir, "commit", "-m", "feat(math): add multiply function")

	// Child 2 creates string_utils.py in its workdir
	child2Str := "def reverse_string(s):\n    return s[::-1]\n"
	if err := os.WriteFile(filepath.Join(job2.WorkDir, "string_utils.py"), []byte(child2Str), 0644); err != nil {
		t.Fatal(err)
	}
	runGit(t, job2.WorkDir, "add", "string_utils.py")
	runGit(t, job2.WorkDir, "commit", "-m", "feat(strings): add reverse_string function")

	// Step 6: Verify main project remains clean and untouched
	mainMath, err := os.ReadFile(filepath.Join(projectDir, "math_utils.py"))
	if err != nil || string(mainMath) != initialPy {
		t.Fatalf("main project math_utils.py was prematurely modified: %s", string(mainMath))
	}
	if _, err := os.Stat(filepath.Join(projectDir, "string_utils.py")); !os.IsNotExist(err) {
		t.Fatal("string_utils.py leaked into main project before merge")
	}

	// Step 7: Follow-up instruction (send_agent) on child 1
	sendResp := h.postJSON("/api/subagents/"+job1.ID+"/send", map[string]string{
		"chat_id": chat,
		"prompt":  "TASK_MULTIPLY_FOLLOWUP: add docstring to multiply function",
	})
	if sendResp.StatusCode != http.StatusOK {
		t.Fatalf("send_agent status: %d", sendResp.StatusCode)
	}
	sendResp.Body.Close()

	// Child 1 commits the docstring in its worktree
	child1MathWithDoc := initialPy + "\ndef multiply(a, b):\n    \"\"\"Multiply two numbers.\"\"\"\n    return a * b\n"
	if err := os.WriteFile(filepath.Join(job1.WorkDir, "math_utils.py"), []byte(child1MathWithDoc), 0644); err != nil {
		t.Fatal(err)
	}
	runGit(t, job1.WorkDir, "add", "math_utils.py")
	runGit(t, job1.WorkDir, "commit", "-m", "docs(math): add docstring to multiply")

	// Wait for child 1 to finish round 2 and await follow-up
	job1After := awaitSubAgentJob(t, h, chat, func(j subagents.Job) bool {
		return j.ID == job1.ID && j.Round == 2 && j.State == subagents.Awaiting
	})
	if job1After.Round != 2 {
		t.Fatalf("expected round 2, got %d", job1After.Round)
	}

	// Step 8: Sequential Merge of Child 1
	mergeResp1 := h.postJSON("/api/subagents/"+job1.ID+"/merge", map[string]string{"chat_id": chat})
	if mergeResp1.StatusCode != http.StatusOK {
		t.Fatalf("merge child 1 status: %d", mergeResp1.StatusCode)
	}
	mergeResp1.Body.Close()

	// Verify Child 1 changes landed on main
	mainMathAfter1, err := os.ReadFile(filepath.Join(projectDir, "math_utils.py"))
	if err != nil || !strings.Contains(string(mainMathAfter1), "def multiply") || !strings.Contains(string(mainMathAfter1), "Multiply two numbers") {
		t.Fatalf("main math_utils.py missing child 1 content: %s", string(mainMathAfter1))
	}

	// Verify child 1 worktree is deleted
	if _, err := os.Stat(job1.WorkDir); !os.IsNotExist(err) {
		t.Fatalf("child 1 worktree %s still exists after successful merge", job1.WorkDir)
	}

	// Close child 1
	closeResp1 := h.postJSON("/api/subagents/"+job1.ID+"/close", map[string]string{"chat_id": chat})
	if closeResp1.StatusCode != http.StatusOK {
		t.Fatalf("close child 1 status: %d", closeResp1.StatusCode)
	}
	closeResp1.Body.Close()

	// Step 9: Sequential Merge of Child 2
	mergeResp2 := h.postJSON("/api/subagents/"+job2.ID+"/merge", map[string]string{"chat_id": chat})
	if mergeResp2.StatusCode != http.StatusOK {
		t.Fatalf("merge child 2 status: %d", mergeResp2.StatusCode)
	}
	mergeResp2.Body.Close()

	// Verify Child 2 changes landed on main
	mainStrAfter2, err := os.ReadFile(filepath.Join(projectDir, "string_utils.py"))
	if err != nil || !strings.Contains(string(mainStrAfter2), "def reverse_string") {
		t.Fatalf("main string_utils.py missing child 2 content: %s", string(mainStrAfter2))
	}

	// Verify child 2 worktree is deleted
	if _, err := os.Stat(job2.WorkDir); !os.IsNotExist(err) {
		t.Fatalf("child 2 worktree %s still exists after successful merge", job2.WorkDir)
	}

	// Close child 2
	closeResp2 := h.postJSON("/api/subagents/"+job2.ID+"/close", map[string]string{"chat_id": chat})
	if closeResp2.StatusCode != http.StatusOK {
		t.Fatalf("close child 2 status: %d", closeResp2.StatusCode)
	}
	closeResp2.Body.Close()

	// Step 10: Final validation on main project
	logOut := runGit(t, projectDir, "log", "--oneline", "-n", "10")
	for _, expectedCommit := range []string{"feat(math)", "docs(math)", "feat(strings)"} {
		if !strings.Contains(logOut, expectedCommit) {
			t.Errorf("main git log missing commit %s:\n%s", expectedCommit, logOut)
		}
	}

	t.Logf("BENCHMARK_SUCCESS: All subagent scenarios verified cleanly on fake project!")
}

func runGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s in %s failed: %v\nOutput: %s", strings.Join(args, " "), dir, err, string(out))
	}
	return string(out)
}
