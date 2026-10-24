package taskloop

import (
	"context"
	"errors"
	"strings"
	"testing"
)

type workerRegressionRunner func(context.Context, SubAgentSpec, bool) (string, error)

func (f workerRegressionRunner) Run(ctx context.Context, spec SubAgentSpec, write bool) (string, error) {
	return f(ctx, spec, write)
}

func TestProcessItem_SubAgentsKeepRulesAndRetryContext(t *testing.T) {
	for _, reviewError := range []bool{false, true} {
		t.Run(map[bool]string{false: "rejected", true: "review-error"}[reviewError], func(t *testing.T) {
			store, err := NewStore(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			list, err := store.Create("chat", "task", []string{"fix [parallel]"})
			if err != nil {
				t.Fatal(err)
			}
			calls := 0
			runner := workerRegressionRunner(func(ctx context.Context, spec SubAgentSpec, write bool) (string, error) {
				calls++
				if !strings.Contains(spec.Task, "PROJECT_RULES") {
					t.Errorf("round %d lost rules: %q", calls, spec.Task)
				}
				if calls == 2 && !strings.Contains(spec.Task, "first worker output") {
					t.Errorf("retry lost previous output: %q", spec.Task)
				}
				if calls == 2 && !strings.Contains(spec.Task, "review explanation") {
					t.Errorf("retry lost feedback: %q", spec.Task)
				}
				return "first worker output", nil
			})
			reviews := 0
			chief := func(context.Context, string, string) (bool, string, error) {
				reviews++
				if reviews == 1 {
					if reviewError {
						return false, "", errors.New("review explanation")
					}
					return false, "review explanation", nil
				}
				return true, "", nil
			}
			e := NewEngine(store, nil, chief, nil, nil, WithSubAgents(NewSubAgentOrchestrator(runner, 1), func(ctx context.Context, item, feedback string) []SubAgentSpec {
				return []SubAgentSpec{{Role: SubRoleCoder, Task: item + feedback}}
			}))
			ok, cancelled, suspended := e.processItem(context.Background(), list.ID, &list.Items[0], "chat", "PROJECT_RULES\n")
			if !ok || cancelled || suspended {
				t.Fatalf("result: %v %v %v", ok, cancelled, suspended)
			}
		})
	}
}

func TestSpawn_FailedCoderStopsReadersAndPreservesCause(t *testing.T) {
	cause := errors.New("429 quota exhausted")
	calls := 0
	o := NewSubAgentOrchestrator(workerRegressionRunner(func(context.Context, SubAgentSpec, bool) (string, error) { calls++; return "partial", cause }), 3)
	_, err := o.Spawn(context.Background(), "task", specsFor(SubRoleCoder, SubRoleReviewer, SubRoleTester))
	if !errors.Is(err, cause) {
		t.Errorf("lost coder cause: %v", err)
	}
	if calls != 1 {
		t.Errorf("ran %d agents after coder failure; want 1", calls)
	}
}

func TestSpawn_CancelledBeforeStartDoesNotRunAgents(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	calls := 0
	o := NewSubAgentOrchestrator(workerRegressionRunner(func(context.Context, SubAgentSpec, bool) (string, error) { calls++; return "", nil }), 1)
	_, err := o.Spawn(ctx, "task", specsFor(SubRoleCoder, SubRoleReviewer))
	if !errors.Is(err, context.Canceled) || calls != 0 {
		t.Fatalf("err=%v calls=%d", err, calls)
	}
}

func TestSpawn_ReadersReceiveBoundedCoderHandoff(t *testing.T) {
	coderOutput := "changed widget.go; test passed\n" + strings.Repeat("detail ", 9000)
	o := NewSubAgentOrchestrator(workerRegressionRunner(func(ctx context.Context, spec SubAgentSpec, write bool) (string, error) {
		if spec.Role == SubRoleCoder {
			return coderOutput, nil
		}
		if !strings.Contains(spec.Task, "changed widget.go; test passed") {
			t.Error("reader did not receive coder handoff")
		}
		if len(spec.Task) >= len(coderOutput) {
			t.Errorf("handoff not bounded: %d bytes", len(spec.Task))
		}
		return "reviewed", nil
	}), 3)
	if _, err := o.Spawn(context.Background(), "task", specsFor(SubRoleCoder, SubRoleReviewer, SubRoleTester)); err != nil {
		t.Fatal(err)
	}
}

func TestSpawn_CancelledAfterCoderDoesNotRunReaders(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	calls := 0
	o := NewSubAgentOrchestrator(workerRegressionRunner(func(context.Context, SubAgentSpec, bool) (string, error) { calls++; cancel(); return "partial", nil }), 1)
	_, err := o.Spawn(ctx, "task", specsFor(SubRoleCoder, SubRoleReviewer))
	if !errors.Is(err, context.Canceled) || calls != 1 {
		t.Fatalf("err=%v calls=%d", err, calls)
	}
}

func TestProcessItem_SubAgentFailureReachesWorkerErrorHandling(t *testing.T) {
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	list, err := store.Create("chat", "task", []string{"fix [parallel]"})
	if err != nil {
		t.Fatal(err)
	}
	cause := errors.New("original worker failure")
	type contextKey struct{}
	ctx := context.WithValue(context.Background(), contextKey{}, "list-specific-model")
	for _, role := range []SubRole{SubRoleCoder, SubRoleReviewer} {
		t.Run(string(role), func(t *testing.T) {
			var observed error
			e := NewEngine(store, nil, func(context.Context, string, string) (bool, string, error) {
				t.Error("chief ran after all workers failed")
				return true, "", nil
			}, nil, nil,
				WithSelfHeal(func(ctx context.Context, listID string, err error) bool { observed = err; return false }),
				WithSubAgents(NewSubAgentOrchestrator(workerRegressionRunner(func(context.Context, SubAgentSpec, bool) (string, error) { return "", cause }), 1), func(got context.Context, item, feedback string) []SubAgentSpec {
					if got.Value(contextKey{}) != "list-specific-model" {
						t.Error("resolver lost list context")
					}
					return specsFor(role)
				}))
			ok, cancelled, suspended := e.processItem(ctx, list.ID, &list.Items[0], "chat", "")
			if ok || cancelled || suspended || !errors.Is(observed, cause) {
				t.Fatalf("ok=%v cancelled=%v suspended=%v cause=%v", ok, cancelled, suspended, observed)
			}
		})
	}
}

func TestProcessItem_VerboseSimpleItemUsesOneWorker(t *testing.T) {
	item := &TaskItem{ID: "item", Text: "Açıklamadaki yazım yanlışını düzelt: " + strings.Repeat("ölçü açıklaması ", 30)}
	workerCalls, resolverCalls, agentCalls, reviewCalls := 0, 0, 0, 0
	e := NewEngine(nil, func(ctx context.Context, chatID, prompt string) (string, error) {
		workerCalls++
		if prompt != "PROJECT_RULES\n"+item.Text {
			t.Errorf("worker lost prompt: %q", prompt)
		}
		return "corrected typo", nil
	}, func(context.Context, string, string) (bool, string, error) {
		reviewCalls++
		return true, "", nil
	}, nil, nil, WithSubAgents(NewSubAgentOrchestrator(workerRegressionRunner(func(context.Context, SubAgentSpec, bool) (string, error) {
		agentCalls++
		return "fanout output", nil
	}), 1), func(context.Context, string, string) []SubAgentSpec {
		resolverCalls++
		return specsFor(SubRoleCoder)
	}))
	ok, cancelled, suspended := e.processItem(context.Background(), "list", item, "chat", "PROJECT_RULES\n")
	if !ok || cancelled || suspended {
		t.Fatalf("result: %v %v %v", ok, cancelled, suspended)
	}
	if workerCalls != 1 || resolverCalls != 0 || agentCalls != 0 || reviewCalls != 1 {
		t.Fatalf("worker=%d resolver=%d subagent=%d chief=%d; want 1/0/0/1", workerCalls, resolverCalls, agentCalls, reviewCalls)
	}
}
