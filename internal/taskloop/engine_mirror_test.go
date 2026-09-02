package taskloop

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestEngine_MirrorsCheckboxOnItemDone(t *testing.T) {
	dir := t.TempDir()
	md := filepath.Join(dir, "Task.md")
	os.WriteFile(md, []byte("- [ ] first thing\n- [ ] second thing\n"), 0644)

	store, _ := NewStore(t.TempDir())
	tl, _ := store.CreateWithMeta("c1", "T", []string{"first thing", "second thing"}, NotifyImportant, md)
	// Record source lines like CreateTaskListFromTaskMd does.
	_ = store.SetItemLine(tl.ID, tl.Items[0].ID, 1)
	_ = store.SetItemLine(tl.ID, tl.Items[1].ID, 2)

	eng := NewEngine(store,
		func(ctx context.Context, chatID, prompt string) (string, error) { return "ok", nil },
		func(ctx context.Context, itemText, workerOutput string) (bool, string, error) { return true, "", nil },
		func(bool) {}, func(string, string) {},
	)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = eng.Start(ctx, tl.ID)
	waitForStatus(t, store, tl.ID, taskListDone, 3*time.Second)

	got, _ := os.ReadFile(md)
	want := "- [x] first thing\n- [x] second thing\n"
	if string(got) != want {
		t.Fatalf("Task.md not mirrored:\n%q\nwant:\n%q", string(got), want)
	}
}

func TestEngine_ParksWaitingUserWhenProvidersExhausted(t *testing.T) {
	store, _ := NewStore(t.TempDir())
	tl, _ := store.Create("c1", "T", []string{"do it"})

	eng := NewEngine(store,
		func(ctx context.Context, chatID, prompt string) (string, error) {
			return "", errors.New("status 401: invalid api key")
		},
		func(ctx context.Context, itemText, workerOutput string) (bool, string, error) { return true, "", nil },
		func(bool) {}, func(string, string) {},
		WithSelfHeal(func(ctx context.Context, listID string, err error) bool {
			return false // no provider left
		}),
	)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = eng.Start(ctx, tl.ID)
	waitForStatus(t, store, tl.ID, taskListWaitingUser, 3*time.Second)

	got, _ := store.Get(tl.ID)
	if got.Items[0].Status != "pending" {
		t.Fatalf("item = %q, want pending (resumable), not stuck", got.Items[0].Status)
	}
}
