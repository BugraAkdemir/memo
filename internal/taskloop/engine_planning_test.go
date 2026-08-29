package taskloop

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestEngine_PlanningPhaseInjectsRulesAndGuidance(t *testing.T) {
	store, _ := NewStore(t.TempDir())
	tl, _ := store.Create("chat-proj", "T", []string{"do the thing"})

	var mu sync.Mutex
	var seenPrompt string
	var phases []string

	engine := NewEngine(
		store,
		func(ctx context.Context, chatID, prompt string) (string, error) {
			mu.Lock()
			seenPrompt = prompt
			mu.Unlock()
			return "worker done", nil
		},
		func(ctx context.Context, itemText, workerOutput string) (bool, string, error) {
			return true, "", nil
		},
		func(v bool) {},
		func(name, data string) {
			if name == "taskloop:planning" {
				mu.Lock()
				phases = append(phases, "planning")
				mu.Unlock()
			}
		},
		WithProjectPathFn(func(chatID string) string {
			if chatID != "chat-proj" {
				t.Errorf("projectPathFn got chatID %q", chatID)
			}
			return "/fake/project"
		}),
		WithRuleReader(func(projectRoot string) (string, error) {
			if projectRoot != "/fake/project" {
				t.Errorf("ruleReader got root %q", projectRoot)
			}
			return "RULE: always run the tests", nil
		}),
		WithSystemGuidance(func() string { return "GUIDANCE: switch provider on auth error" }),
	)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := engine.Start(ctx, tl.ID); err != nil {
		t.Fatalf("Start: %v", err)
	}

	deadline := time.After(3 * time.Second)
	for {
		got, _ := store.Get(tl.ID)
		if got.Status == taskListDone {
			break
		}
		select {
		case <-deadline:
			t.Fatalf("list did not finish; status=%s", got.Status)
		case <-time.After(20 * time.Millisecond):
		}
	}

	mu.Lock()
	prompt := seenPrompt
	sawPlanning := len(phases) > 0
	mu.Unlock()

	if !sawPlanning {
		t.Error("no taskloop:planning event emitted")
	}
	if !strings.Contains(prompt, "RULE: always run the tests") {
		t.Errorf("worker prompt missing repo rules:\n%s", prompt)
	}
	if !strings.Contains(prompt, "GUIDANCE: switch provider on auth error") {
		t.Errorf("worker prompt missing memo-system guidance:\n%s", prompt)
	}
	if !strings.Contains(prompt, "do the thing") {
		t.Errorf("worker prompt missing the item text:\n%s", prompt)
	}
}

func TestEngine_PlanConfigRunsOnceBeforeItems(t *testing.T) {
	store, _ := NewStore(t.TempDir())
	tl, _ := store.Create("chat-x", "T", []string{"item one", "item two"})

	var mu sync.Mutex
	planConfigCalls := 0
	var planConfigItems []string
	firstWorkerAt := 0
	workerCalls := 0

	engine := NewEngine(
		store,
		func(ctx context.Context, chatID, prompt string) (string, error) {
			mu.Lock()
			workerCalls++
			if firstWorkerAt == 0 {
				firstWorkerAt = workerCalls
			}
			mu.Unlock()
			return "ok", nil
		},
		func(ctx context.Context, itemText, workerOutput string) (bool, string, error) { return true, "", nil },
		func(v bool) {},
		nil,
		WithPlanConfig(func(ctx context.Context, listID, chatID string, items []string) error {
			mu.Lock()
			planConfigCalls++
			planConfigItems = append([]string(nil), items...)
			w := workerCalls
			mu.Unlock()
			if w != 0 {
				t.Errorf("plan-config ran after %d worker turns; must run first", w)
			}
			if chatID != "chat-x" {
				t.Errorf("plan-config chatID = %q", chatID)
			}
			return nil
		}),
	)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = engine.Start(ctx, tl.ID)
	waitForStatus(t, store, tl.ID, taskListDone, 3*time.Second)

	mu.Lock()
	defer mu.Unlock()
	if planConfigCalls != 1 {
		t.Fatalf("planConfigCalls = %d, want 1", planConfigCalls)
	}
	if len(planConfigItems) != 2 || planConfigItems[0] != "item one" {
		t.Fatalf("plan-config items = %v", planConfigItems)
	}
}

func TestEngine_PlanConfigErrorDoesNotBlock(t *testing.T) {
	store, _ := NewStore(t.TempDir())
	tl, _ := store.Create("chat1", "T", []string{"item"})

	engine := NewEngine(
		store,
		func(ctx context.Context, chatID, prompt string) (string, error) { return "ok", nil },
		func(ctx context.Context, itemText, workerOutput string) (bool, string, error) { return true, "", nil },
		func(v bool) {},
		nil,
		WithPlanConfig(func(ctx context.Context, listID, chatID string, items []string) error {
			return errors.New("plan-config exploded")
		}),
	)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = engine.Start(ctx, tl.ID)
	waitForStatus(t, store, tl.ID, taskListDone, 3*time.Second)
}

func TestEngine_PlanningPhaseNoHooksIsPlainItemText(t *testing.T) {
	store, _ := NewStore(t.TempDir())
	tl, _ := store.Create("chat1", "T", []string{"bare item"})

	var mu sync.Mutex
	var seenPrompt string
	engine := NewEngine(
		store,
		func(ctx context.Context, chatID, prompt string) (string, error) {
			mu.Lock()
			seenPrompt = prompt
			mu.Unlock()
			return "ok", nil
		},
		func(ctx context.Context, itemText, workerOutput string) (bool, string, error) { return true, "", nil },
		func(v bool) {},
		nil,
	)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = engine.Start(ctx, tl.ID)

	deadline := time.After(3 * time.Second)
	for {
		got, _ := store.Get(tl.ID)
		if got.Status == taskListDone {
			break
		}
		select {
		case <-deadline:
			t.Fatalf("list did not finish; status=%s", got.Status)
		case <-time.After(20 * time.Millisecond):
		}
	}

	mu.Lock()
	defer mu.Unlock()
	if seenPrompt != "bare item" {
		t.Fatalf("with no planning hooks the prompt should be the raw item text, got %q", seenPrompt)
	}
}
