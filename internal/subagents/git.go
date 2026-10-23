package subagents

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func git(ctx context.Context, dir string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", append([]string{"-C", dir}, args...)...)
	b, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(string(b)))
	}
	return strings.TrimSpace(string(b)), nil
}
func cleanRepo(ctx context.Context, dir string) error {
	s, err := git(ctx, dir, "status", "--porcelain", "--untracked-files=all")
	if err != nil {
		return err
	}
	if s != "" {
		return errors.New("Git worktree must be clean, including untracked files")
	}
	return nil
}
func mainRepo(ctx context.Context, dir string) error {
	for _, marker := range []string{"MERGE_HEAD", "CHERRY_PICK_HEAD", "REVERT_HEAD", "rebase-merge", "rebase-apply"} {
		path, err := git(ctx, dir, "rev-parse", "--git-path", marker)
		if err != nil {
			return err
		}
		if !filepath.IsAbs(path) {
			path = filepath.Join(dir, path)
		}
		if _, err = os.Stat(path); err == nil {
			return errors.New("project has an unfinished Git operation; finish it before subagent integration")
		} else if !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	b, err := git(ctx, dir, "symbolic-ref", "--quiet", "--short", "HEAD")
	if err != nil {
		return err
	}
	if b != "main" {
		return errors.New("project must be on main before isolated work or merge")
	}
	return cleanRepo(ctx, dir)
}
func createWorkspace(ctx context.Context, dir string, j *Job) error {
	if j.ProjectDir == "" {
		return errors.New("isolated subagent requires a Git project")
	}
	root, err := git(ctx, j.ProjectDir, "rev-parse", "--show-toplevel")
	if err != nil {
		return err
	}
	root, err = filepath.Abs(root)
	if err != nil {
		return err
	}
	j.ProjectDir = root
	if err = mainRepo(ctx, root); err != nil {
		return err
	}
	base, err := git(ctx, root, "rev-parse", "HEAD")
	if err != nil {
		return err
	}
	j.BaseCommit = base
	j.Branch = "memo-agent-" + j.ID
	j.WorkDir = filepath.Join(dir, "worktrees", j.ID)
	j.WorkDir, err = filepath.Abs(j.WorkDir)
	if err != nil {
		return err
	}
	if err = os.MkdirAll(filepath.Dir(j.WorkDir), 0700); err != nil {
		return err
	}
	_, err = git(ctx, root, "worktree", "add", "-b", j.Branch, j.WorkDir, base)
	return err
}

// Merge serializes integration into main. Git's merge --no-commit allows an
// unsuccessful attempt to be aborted without leaving main in a conflict state.
// Only successful, ancestry-verified integration permits workspace removal.
func (m *Manager) Merge(ctx context.Context, chat, id string) error {
	return m.MergeValidated(ctx, chat, id, func(ctx context.Context, dir string) error {
		_, err := git(ctx, dir, "diff", "--check", "HEAD")
		return err
	})
}

// MergeValidated validates the combined tree before creating the merge commit.
// Validation failures abort integration and keep the child's branch and files.
func (m *Manager) MergeValidated(ctx context.Context, chat, id string, validate func(context.Context, string) error) error {
	if validate == nil {
		return errors.New("merge validator is required")
	}
	m.gitMu.Lock()
	defer m.gitMu.Unlock()
	m.mu.Lock()
	j, err := m.ownedLocked(chat, id)
	if err != nil {
		m.mu.Unlock()
		return err
	}
	snapshot := clone(*j)
	if j.Isolated && j.Merged {
		m.mu.Unlock()
		return cleanupWorkspace(ctx, snapshot)
	}
	if !j.Isolated || j.State != Awaiting {
		m.mu.Unlock()
		return errors.New("task must be completed and isolated before merge")
	}
	for activeID := range m.active {
		active := m.data.Jobs[activeID]
		if active.Write && !active.Isolated && pathsOverlap(active.WorkDir, snapshot.ProjectDir) {
			m.mu.Unlock()
			return errors.New("shared project writer is still running; retry merge after it finishes")
		}
	}
	m.integrating[snapshot.ProjectDir] = true
	m.mu.Unlock()
	defer func() { m.mu.Lock(); delete(m.integrating, snapshot.ProjectDir); m.scheduleLocked(); m.mu.Unlock() }()
	if err = mainRepo(ctx, snapshot.ProjectDir); err != nil {
		return err
	}
	if err = cleanRepo(ctx, snapshot.WorkDir); err != nil {
		return err
	}
	branch, err := git(ctx, snapshot.WorkDir, "symbolic-ref", "--quiet", "--short", "HEAD")
	if err != nil {
		return err
	}
	if branch != snapshot.Branch {
		return errors.New("subagent worktree branch changed")
	}
	if _, err = git(ctx, snapshot.ProjectDir, "merge-base", "--is-ancestor", snapshot.BaseCommit, snapshot.Branch); err != nil {
		return errors.New("subagent branch does not descend from its original base")
	}
	// Refuse child rewrites of main's history and preserve a failed branch.
	if _, err = git(ctx, snapshot.ProjectDir, "merge", "--no-ff", "--no-commit", snapshot.Branch); err != nil {
		_, abortErr := git(context.WithoutCancel(ctx), snapshot.ProjectDir, "merge", "--abort")
		if abortErr != nil {
			return fmt.Errorf("%w (merge abort: %v)", err, abortErr)
		}
		return err
	}
	if err = validate(ctx, snapshot.ProjectDir); err != nil {
		_, abortErr := git(context.WithoutCancel(ctx), snapshot.ProjectDir, "merge", "--abort")
		if abortErr != nil {
			if _, stateErr := git(context.WithoutCancel(ctx), snapshot.ProjectDir, "rev-parse", "--verify", "MERGE_HEAD"); stateErr == nil {
				return fmt.Errorf("validation failed: %w (abort: %v)", err, abortErr)
			}
		}
		return fmt.Errorf("merge validation failed: %w", err)
	}
	// No merge state exists if all changes were already integrated.
	if _, stateErr := git(context.WithoutCancel(ctx), snapshot.ProjectDir, "rev-parse", "--verify", "MERGE_HEAD"); stateErr == nil {
		if _, err = git(ctx, snapshot.ProjectDir, "commit", "-m", "feat(agent): integrate subagent task "+snapshot.ID); err != nil {
			_, _ = git(context.WithoutCancel(ctx), snapshot.ProjectDir, "merge", "--abort")
			return err
		}
	}
	if _, err = git(ctx, snapshot.ProjectDir, "merge-base", "--is-ancestor", snapshot.Branch, "main"); err != nil {
		return errors.New("merge ancestry verification failed; workspace retained")
	}
	m.mu.Lock()
	j = m.data.Jobs[id]
	j.Merged = true
	if err = m.saveLocked(); err != nil {
		j.Merged = false
		m.mu.Unlock()
		return err
	}
	m.mu.Unlock()
	return cleanupWorkspace(ctx, snapshot)
}

// Retrying cleanup after a crash is safe: first prove main contains the branch,
// and never force-remove a dirty worktree or an unmerged branch.
func cleanupWorkspace(ctx context.Context, j Job) error {
	ref, err := git(ctx, j.ProjectDir, "for-each-ref", "--format=%(refname)", "refs/heads/"+j.Branch)
	if err != nil {
		return err
	}
	if ref == "" {
		if _, err = os.Stat(j.WorkDir); err == nil {
			return errors.New("worktree still exists but its recorded branch is missing; retaining task history")
		}
		if !errors.Is(err, os.ErrNotExist) {
			return err
		}
		return nil
	}
	if _, err = git(ctx, j.ProjectDir, "merge-base", "--is-ancestor", j.Branch, "main"); err != nil {
		return errors.New("branch is not integrated into main; retaining workspace")
	}
	if _, err = os.Stat(j.WorkDir); err == nil {
		if err = cleanRepo(ctx, j.WorkDir); err != nil {
			return err
		}
		if _, err = git(ctx, j.ProjectDir, "worktree", "remove", j.WorkDir); err != nil {
			return err
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	_, err = git(ctx, j.ProjectDir, "branch", "-d", j.Branch)
	return err
}

func pathsOverlap(a, b string) bool {
	if a == "" || b == "" {
		return a == b
	}
	relative, err := filepath.Rel(a, b)
	if err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return true
	}
	relative, err = filepath.Rel(b, a)
	return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}
func discardUnusedWorkspace(ctx context.Context, j Job) error {
	head, err := git(ctx, j.WorkDir, "rev-parse", "HEAD")
	if err != nil {
		return err
	}
	if head != j.BaseCommit {
		return errors.New("unused workspace changed; retained")
	}
	if err = cleanRepo(ctx, j.WorkDir); err != nil {
		return err
	}
	if _, err = git(ctx, j.ProjectDir, "worktree", "remove", j.WorkDir); err != nil {
		return err
	}
	_, err = git(ctx, j.ProjectDir, "branch", "-d", j.Branch)
	return err
}
