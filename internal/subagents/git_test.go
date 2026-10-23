package subagents

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func repository(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	ctx := context.Background()
	for _, args := range [][]string{{"init", "-b", "main"}, {"config", "user.name", "Memo Test"}, {"config", "user.email", "test@example.invalid"}} {
		if _, err := git(ctx, dir, args...); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "file"), []byte("base\n"), 0600); err != nil {
		t.Fatal(err)
	}
	commitFile(t, dir, "base")
	return dir
}
func commitFile(t *testing.T, dir, msg string) {
	t.Helper()
	for _, args := range [][]string{{"add", "."}, {"commit", "-m", msg}} {
		if _, err := git(context.Background(), dir, args...); err != nil {
			t.Fatal(err)
		}
	}
}
func TestIsolationMergeAndConflict(t *testing.T) {
	repo := repository(t)
	m, err := Open(t.TempDir(), 3, func(context.Context, Job, func(Event)) (RunResult, error) { return RunResult{}, nil })
	if err != nil {
		t.Fatal(err)
	}
	defer m.Shutdown(context.Background())
	a, err := m.Create(CreateRequest{ChatID: "chat", Prompt: "a", ProjectDir: repo, Isolated: true})
	if err != nil {
		t.Fatal(err)
	}
	b, err := m.Create(CreateRequest{ChatID: "chat", Prompt: "b", ProjectDir: repo, Isolated: true})
	if err != nil {
		t.Fatal(err)
	}
	for _, j := range []Job{a, b} {
		if !strings.HasPrefix(j.Branch, "memo-") || j.WorkDir == repo {
			t.Fatal(j)
		}
		if err := os.WriteFile(filepath.Join(j.WorkDir, "file"), []byte(j.ID+"\n"), 0600); err != nil {
			t.Fatal(err)
		}
		commitFile(t, j.WorkDir, j.ID)
	}
	original, _ := os.ReadFile(filepath.Join(repo, "file"))
	if string(original) != "base\n" {
		t.Fatal("child edited main")
	}
	if err = m.Start(); err != nil {
		t.Fatal(err)
	}
	eventually(t, func() bool {
		aa, _ := m.Get("chat", a.ID)
		bb, _ := m.Get("chat", b.ID)
		return aa.State == Awaiting && bb.State == Awaiting
	})
	if err = m.Merge(context.Background(), "chat", a.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = os.Stat(a.WorkDir); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("merged worktree retained", err)
	}
	if _, err = git(context.Background(), repo, "show-ref", "--verify", "refs/heads/"+a.Branch); err == nil {
		t.Fatal("merged branch retained")
	}
	if err = m.Merge(context.Background(), "chat", b.ID); err == nil {
		t.Fatal("expected conflict")
	}
	if _, err = os.Stat(b.WorkDir); err != nil {
		t.Fatal("conflicted worktree removed", err)
	}
	if err = cleanRepo(context.Background(), repo); err != nil {
		t.Fatal("main left dirty", err)
	}
	if err = m.CloseJob("chat", b.ID); err == nil {
		t.Fatal("unmerged work discarded")
	}
}
func TestDirtyAndNonGitRejected(t *testing.T) {
	m, err := Open(t.TempDir(), 1, func(context.Context, Job, func(Event)) (RunResult, error) { return RunResult{}, nil })
	if err != nil {
		t.Fatal(err)
	}
	defer m.Shutdown(context.Background())
	if _, err = m.Create(CreateRequest{ChatID: "chat", Prompt: "task", ProjectDir: t.TempDir(), Isolated: true}); err == nil {
		t.Fatal("non-Git isolation accepted")
	}
	repo := repository(t)
	if err = os.WriteFile(filepath.Join(repo, "untracked"), []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = m.Create(CreateRequest{ChatID: "chat", Prompt: "task", ProjectDir: repo, Isolated: true}); err == nil {
		t.Fatal("dirty base accepted")
	}
}
func TestMergeValidationFailureRetainsWork(t *testing.T) {
	repo := repository(t)
	m, err := Open(t.TempDir(), 1, func(context.Context, Job, func(Event)) (RunResult, error) { return RunResult{}, nil })
	if err != nil {
		t.Fatal(err)
	}
	defer m.Shutdown(context.Background())
	j, err := m.Create(CreateRequest{ChatID: "chat", Prompt: "task", ProjectDir: repo, Isolated: true})
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(j.WorkDir, "added"), []byte("child\n"), 0600); err != nil {
		t.Fatal(err)
	}
	commitFile(t, j.WorkDir, "child")
	if err = m.Start(); err != nil {
		t.Fatal(err)
	}
	eventually(t, func() bool { jj, _ := m.Get("chat", j.ID); return jj.State == Awaiting })
	before, _ := git(context.Background(), repo, "rev-parse", "HEAD")
	err = m.MergeValidated(context.Background(), "chat", j.ID, func(ctx context.Context, dir string) error {
		if _, e := os.Stat(filepath.Join(dir, "added")); e != nil {
			t.Fatal("validator did not see combined tree")
		}
		return errors.New("tests failed")
	})
	if err == nil {
		t.Fatal("validation ignored")
	}
	after, _ := git(context.Background(), repo, "rev-parse", "HEAD")
	if before != after {
		t.Fatal("failed validation committed")
	}
	if err = cleanRepo(context.Background(), repo); err != nil {
		t.Fatal(err)
	}
	if _, err = os.Stat(j.WorkDir); err != nil {
		t.Fatal(err)
	}
}
func TestMergedWorkspaceCleanupCanBeRetried(t *testing.T) {
	repo := repository(t)
	m, err := Open(t.TempDir(), 1, func(context.Context, Job, func(Event)) (RunResult, error) { return RunResult{}, nil })
	if err != nil {
		t.Fatal(err)
	}
	defer m.Shutdown(context.Background())
	j, err := m.Create(CreateRequest{ChatID: "chat", Prompt: "task", ProjectDir: repo, Isolated: true})
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(j.WorkDir, "added"), []byte("child\n"), 0600); err != nil {
		t.Fatal(err)
	}
	commitFile(t, j.WorkDir, "child")
	if err = m.Start(); err != nil {
		t.Fatal(err)
	}
	eventually(t, func() bool { jj, _ := m.Get("chat", j.ID); return jj.State == Awaiting })
	dirty := filepath.Join(j.WorkDir, "unfinished")
	err = m.MergeValidated(context.Background(), "chat", j.ID, func(context.Context, string) error { return os.WriteFile(dirty, []byte("retain"), 0600) })
	if err == nil {
		t.Fatal("dirty cleanup accepted")
	}
	jj, _ := m.Get("chat", j.ID)
	if !jj.Merged {
		t.Fatal("successful integration not recorded")
	}
	if err = m.CloseJob("chat", j.ID); err == nil {
		t.Fatal("dirty worktree history discarded")
	}
	if err = os.Remove(dirty); err != nil {
		t.Fatal(err)
	}
	if err = m.Merge(context.Background(), "chat", j.ID); err != nil {
		t.Fatal("cleanup retry failed", err)
	}
	if err = m.CloseJob("chat", j.ID); err != nil {
		t.Fatal(err)
	}
	jj, _ = m.Get("chat", j.ID)
	if jj.State != Closed || len(jj.Messages) > 0 || len(jj.Events) > 0 || jj.Prompt != "" {
		t.Fatal("transient data retained", jj)
	}
}
func TestMergeRefusesActiveSharedWriter(t *testing.T) {
	repo := repository(t)
	m, err := Open(t.TempDir(), 2, func(ctx context.Context, j Job, emit func(Event)) (RunResult, error) {
		if !j.Isolated {
			<-ctx.Done()
			return RunResult{}, ctx.Err()
		}
		return RunResult{}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	defer m.Shutdown(context.Background())
	j, err := m.Create(CreateRequest{ChatID: "chat", Prompt: "isolated", ProjectDir: repo, Isolated: true})
	if err != nil {
		t.Fatal(err)
	}
	if err = m.Start(); err != nil {
		t.Fatal(err)
	}
	eventually(t, func() bool { jj, _ := m.Get("chat", j.ID); return jj.State == Awaiting })
	writer, err := m.Create(CreateRequest{ChatID: "chat", Prompt: "write", ProjectDir: repo, Write: true})
	if err != nil {
		t.Fatal(err)
	}
	if err = m.Merge(context.Background(), "chat", j.ID); err == nil {
		t.Fatal("merge raced shared writer")
	}
	if err = m.Cancel("chat", writer.ID); err != nil {
		t.Fatal(err)
	}
}
