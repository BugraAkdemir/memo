// SPDX-License-Identifier: AGPL-3.0-or-later

package e2e

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"memo/internal/subagents"
)

// The provider alone is scripted. Children write, test and commit through real
// agent tools; main spawns, follows up, merges and closes through its tools.
// Gates keep two writers active together and finish child 2 while main is
// reviewing child 1, so the durable FIFO and lack of overlapping reviews are
// assertions rather than assumptions about scheduler timing.
func TestBenchmark_SubagentsFakeProject(t *testing.T) {
	project := t.TempDir()
	runGit(t, project, "init", "-b", "main")
	runGit(t, project, "config", "user.name", "Benchmark Engineer")
	runGit(t, project, "config", "user.email", "benchmark@example.test")
	runGit(t, project, "config", "commit.gpgsign", "false")
	initial := "package sample\n\nfunc Add(a, b int) int { return a + b }\n"
	for name, content := range map[string]string{
		"go.mod":       "module example.test/subagent-benchmark\n\ngo 1.20\n",
		"base.go":      initial,
		"base_test.go": "package sample\nimport \"testing\"\nfunc TestAdd(t *testing.T) { if Add(2, 3) != 5 { t.Fatal(\"add\") } }\n",
	} {
		if err := os.WriteFile(filepath.Join(project, name), []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	runGit(t, project, "add", ".")
	runGit(t, project, "commit", "-m", "chore: initialize fixture")

	h := NewHarness(t)
	h.SetAgentEnabled(true)
	h.SetAutoPermission(true)
	chat := h.NewAgentChat(project)
	gate := func() (<-chan struct{}, func()) {
		ch := make(chan struct{})
		var once sync.Once
		release := func() { once.Do(func() { close(ch) }) }
		t.Cleanup(release) // release scripted requests before Harness shutdown
		return ch, release
	}
	mathFinish, finishMath := gate()
	stringFinish, finishString := gate()
	firstReviewFinish, finishFirstReview := gate()
	followupFinish, finishFollowup := gate()
	prepared := make(chan string, 3)
	reviews := make(chan subagents.Completion, 4)
	var seenMu sync.Mutex
	seen := map[uint64]bool{}

	h.Fake.Script = func(_ int, req FakeChatRequest) FakeChatResponse {
		last := lastSubAgentUser(req)
		if childRequest(req) {
			label, source, testSource, commit := "math", "math.go", "math_test.go", "feat(math): add multiply"
			code := "package sample\n\nfunc Multiply(a, b int) int { return a * b }\n"
			testCode := "package sample\nimport \"testing\"\nfunc TestMultiply(t *testing.T) { if Multiply(6, 7) != 42 { t.Fatal(\"multiply\") } }\n"
			finish := mathFinish
			if strings.Contains(last, "TASK_MULTIPLY_FOLLOWUP") {
				label, commit, finish = "followup", "docs(math): document multiply", followupFinish
				code = "package sample\n\n// Multiply returns the product of two integers.\nfunc Multiply(a, b int) int { return a * b }\n"
				if !strings.Contains(string(req.Raw), "RESULT_math") || !strings.Contains(string(req.Raw), "TASK_MULTIPLY:") {
					t.Error("follow-up lost the child's original task/result history")
				}
			} else if strings.Contains(last, "TASK_REVERSE:") {
				label, source, testSource, commit, finish = "string", "string.go", "string_test.go", "feat(strings): add reverse", stringFinish
				code = "package sample\n\nfunc Reverse(s string) string { r := []rune(s); for i, j := 0, len(r)-1; i < j; i, j = i+1, j-1 { r[i], r[j] = r[j], r[i] }; return string(r) }\n"
				testCode = "package sample\nimport \"testing\"\nfunc TestReverse(t *testing.T) { if Reverse(\"Memo\") != \"omeM\" { t.Fatal(\"reverse\") } }\n"
			}
			steps := []FakeToolCall{
				benchmarkCall(label+"-write", "write_file", map[string]any{"path": source, "content": code}),
				benchmarkCall(label+"-test-write", "write_file", map[string]any{"path": testSource, "content": testCode}),
				benchmarkCall(label+"-test", "run_command", map[string]any{"command": "go test ./..."}),
				benchmarkCall(label+"-stage", "run_command", map[string]any{"command": "git add " + source + " " + testSource}),
				benchmarkCall(label+"-commit", "run_command", map[string]any{"command": "git commit -m \"" + commit + "\""}),
			}
			if response, done := benchmarkToolSequence(t, req, steps); !done {
				return response
			}
			results := benchmarkToolResults(req)
			if !strings.Contains(results[label+"-test"], "ok") || !strings.Contains(results[label+"-commit"], commit) {
				t.Errorf("%s child did not test and commit successfully: %v", label, results)
			}
			prepared <- label
			<-finish
			return FakeChatResponse{Text: "RESULT_" + label}
		}
		if len(req.Tools) == 0 {
			return FakeChatResponse{Text: "fixture"} // title/metadata requests
		}
		if strings.Contains(last, "<memo-subagent-result>") {
			var completion subagents.Completion
			start, end := strings.Index(last, "\n{"), strings.LastIndex(last, "\n</memo-subagent-result>")
			if start < 0 || end <= start || json.Unmarshal([]byte(last[start+1:end]), &completion) != nil {
				t.Errorf("invalid completion envelope: %s", last)
				return FakeChatResponse{Text: "invalid completion"}
			}
			seenMu.Lock()
			first := !seen[completion.ID]
			seen[completion.ID] = true
			seenMu.Unlock()
			if first {
				reviews <- completion
			}
			prefix := fmt.Sprintf("review-%d", completion.ID)
			if completion.Result == "RESULT_math" {
				<-firstReviewFinish
				step := benchmarkCall(prefix+"-followup", "send_agent", map[string]any{"id": completion.JobID, "prompt": "TASK_MULTIPLY_FOLLOWUP: document Multiply, retest and commit"})
				if response, done := benchmarkToolSequence(t, req, []FakeToolCall{step}); !done {
					return response
				}
			} else {
				steps := []FakeToolCall{
					benchmarkCall(prefix+"-verify", "run_command", map[string]any{"command": "go test ./..."}),
					benchmarkCall(prefix+"-merge", "merge_agent", map[string]any{"id": completion.JobID}),
					benchmarkCall(prefix+"-close", "close_agent", map[string]any{"id": completion.JobID}),
				}
				if response, done := benchmarkToolSequence(t, req, steps); !done {
					return response
				}
				results := benchmarkToolResults(req)
				if !strings.Contains(results[prefix+"-merge"], `"status":"merged"`) || !strings.Contains(results[prefix+"-close"], `"status":"closed"`) {
					t.Errorf("main did not merge and close child: %v", results)
				}
			}
			return FakeChatResponse{Text: "REVIEWED_" + completion.Result}
		}
		if strings.Contains(last, "DELEGATE_BENCHMARK_TASKS") {
			steps := []FakeToolCall{
				benchmarkCall("spawn-math", "spawn_agent", map[string]any{"prompt": "TASK_MULTIPLY: implement Multiply with tests and commit", "write": true, "isolate": true}),
				benchmarkCall("spawn-string", "spawn_agent", map[string]any{"prompt": "TASK_REVERSE: implement Reverse with tests and commit", "write": true, "isolate": true}),
			}
			if response, done := benchmarkToolSequence(t, req, steps); !done {
				return response
			}
			return FakeChatResponse{Text: "Both children delegated."}
		}
		return FakeChatResponse{Text: "ready"}
	}

	for _, ev := range h.SendMessageStream(chat, "DELEGATE_BENCHMARK_TASKS") {
		if ev.Error != "" {
			t.Fatalf("main stream: %s", ev.Error)
		}
	}
	ready := map[string]bool{}
	for len(ready) < 2 {
		ready[awaitSubAgentValue(t, prepared)] = true
	}
	if !ready["math"] || !ready["string"] {
		t.Fatalf("parallel children did not both prepare their changes: %v", ready)
	}
	job1 := awaitSubAgentJob(t, h, chat, func(j subagents.Job) bool {
		return strings.Contains(j.Prompt, "TASK_MULTIPLY:") && j.State == subagents.Running
	})
	job2 := awaitSubAgentJob(t, h, chat, func(j subagents.Job) bool {
		return strings.Contains(j.Prompt, "TASK_REVERSE:") && j.State == subagents.Running
	})
	if !strings.HasPrefix(job1.Branch, "memo-") || !strings.HasPrefix(job2.Branch, "memo-") || job1.Branch == job2.Branch || job1.WorkDir == job2.WorkDir || job1.WorkDir == project || job2.WorkDir == project {
		t.Fatalf("children are not isolated: %+v / %+v", job1, job2)
	}
	for _, name := range []string{"math.go", "string.go"} {
		if _, err := os.Stat(filepath.Join(project, name)); !os.IsNotExist(err) {
			t.Fatalf("child file %s leaked into main before merge", name)
		}
	}
	if got := runGit(t, project, "status", "--porcelain"); got != "" {
		t.Fatalf("main modified before merge: %s", got)
	}

	finishMath()
	first := awaitSubAgentValue(t, reviews)
	if first.JobID != job1.ID || first.Round != 1 || first.Result != "RESULT_math" {
		t.Fatalf("first review=%+v", first)
	}
	// Child 2 completes while the first main review is still blocked. Its
	// completion must remain pending, without starting a concurrent review.
	finishString()
	awaitSubAgentJob(t, h, chat, func(j subagents.Job) bool {
		return j.ID == job2.ID && j.State == subagents.Awaiting && j.ResultDelivery == "pending"
	})
	select {
	case c := <-reviews:
		t.Fatalf("main review overlapped the blocked first review: %+v", c)
	default:
	}
	finishFirstReview()
	second := awaitSubAgentValue(t, reviews)
	if second.JobID != job2.ID || second.Result != "RESULT_string" || second.ID <= first.ID {
		t.Fatalf("FIFO completion order broken: first=%+v second=%+v", first, second)
	}
	awaitSubAgentSavedReply(t, h, chat, "REVIEWED_RESULT_string")
	if got := awaitSubAgentValue(t, prepared); got != "followup" {
		t.Fatalf("main did not reuse math child for follow-up: %s", got)
	}
	finishFollowup()
	third := awaitSubAgentValue(t, reviews)
	if third.JobID != job1.ID || third.Round != 2 || third.Result != "RESULT_followup" || third.ID <= second.ID {
		t.Fatalf("follow-up identity/order broken: %+v", third)
	}
	awaitSubAgentSavedReply(t, h, chat, "REVIEWED_RESULT_followup")

	for _, job := range []subagents.Job{job1, job2} {
		awaitSubAgentJob(t, h, chat, func(j subagents.Job) bool { return j.ID == job.ID && j.State == subagents.Closed && j.Merged })
		if _, err := os.Stat(job.WorkDir); !os.IsNotExist(err) {
			t.Errorf("merged worktree remains: %s", job.WorkDir)
		}
		if branches := runGit(t, project, "branch", "--list", job.Branch); strings.TrimSpace(branches) != "" {
			t.Errorf("merged child branch remains: %s", branches)
		}
	}
	// No duplicate main review may be hidden by the buffered channel.
	select {
	case c := <-reviews:
		t.Fatalf("duplicate/unexpected main review: %+v", c)
	default:
	}
	b, err := os.ReadFile(filepath.Join(project, "math.go"))
	if err != nil || !strings.Contains(string(b), "// Multiply returns") {
		t.Fatalf("main lost follow-up edits: %s (%v)", b, err)
	}
	cmd := exec.Command("go", "test", "./...")
	cmd.Dir = project
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("merged project tests: %v\n%s", err, out)
	}
	log := runGit(t, project, "log", "--format=%s", "--reverse")
	for _, subject := range []string{"feat(math): add multiply", "docs(math): document multiply", "feat(strings): add reverse"} {
		if !strings.Contains(log, subject) {
			t.Errorf("missing child commit %q in main: %s", subject, log)
		}
	}
	merge2, merge1 := strings.Index(log, "integrate subagent task "+job2.ID), strings.Index(log, "integrate subagent task "+job1.ID)
	if merge2 < 0 || merge1 <= merge2 {
		t.Errorf("main merges did not follow accepted-result order: %s", log)
	}
	t.Log("verified real child write/test/commit tools, main follow-up/merge/close tools, FIFO reviews and branch/worktree cleanup")
}

func benchmarkCall(id, name string, args map[string]any) FakeToolCall {
	b, err := json.Marshal(args)
	if err != nil {
		panic(err)
	}
	return FakeToolCall{ID: id, Name: name, Arguments: string(b)}
}

// Only results since the current user message belong to this turn. In
// particular, main's earlier spawn/merge calls cannot satisfy a new review.
func benchmarkToolResults(req FakeChatRequest) map[string]string {
	results := map[string]string{}
	for _, raw := range req.Messages {
		var m struct {
			Role, Content string
			ToolCallID    string `json:"tool_call_id"`
		}
		if json.Unmarshal(raw, &m) != nil {
			continue
		}
		if m.Role == "user" {
			clear(results)
		}
		if m.Role == "tool" {
			results[m.ToolCallID] = m.Content
		}
	}
	return results
}

func benchmarkToolSequence(t *testing.T, req FakeChatRequest, steps []FakeToolCall) (FakeChatResponse, bool) {
	t.Helper()
	results := benchmarkToolResults(req)
	for _, call := range steps {
		result, ok := results[call.ID]
		if !ok {
			return FakeChatResponse{ToolCalls: []FakeToolCall{call}}, false
		}
		if strings.HasPrefix(result, "Error:") || strings.Contains(result, "Command failed") || strings.Contains(result, "timed out") {
			t.Errorf("agent tool %s (%s) failed: %s", call.ID, call.Name, result)
			return FakeChatResponse{Text: "tool failed"}, false
		}
	}
	return FakeChatResponse{}, true
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
