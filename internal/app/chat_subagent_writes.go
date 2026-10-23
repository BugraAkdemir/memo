package app

import (
	"context"
	"path/filepath"
	"strings"
	"sync"
)

type projectMutationQueue struct {
	mu      sync.Mutex
	pending []*projectMutationTicket
	active  map[*projectMutationTicket]bool
}
type projectMutationTicket struct {
	q     *projectMutationQueue
	path  string
	ready chan struct{}
	once  sync.Once
}

func projectMutationOverlap(a, b string) bool {
	rel, err := filepath.Rel(a, b)
	if err == nil && (rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)))) {
		return true
	}
	rel, err = filepath.Rel(b, a)
	return err == nil && (rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))))
}
func (q *projectMutationQueue) reserve(path string) *projectMutationTicket {
	q.mu.Lock()
	defer q.mu.Unlock()
	t := &projectMutationTicket{q: q, path: path, ready: make(chan struct{})}
	q.pending = append(q.pending, t)
	q.schedule()
	return t
}
func (q *projectMutationQueue) schedule() {
	if q.active == nil {
		q.active = map[*projectMutationTicket]bool{}
	}
	var blocked []*projectMutationTicket
	for _, t := range q.pending {
		conflict := false
		for active := range q.active {
			if projectMutationOverlap(t.path, active.path) {
				conflict = true
				break
			}
		}
		if !conflict {
			for _, prior := range blocked {
				if projectMutationOverlap(t.path, prior.path) {
					conflict = true
					break
				}
			}
		}
		if conflict {
			blocked = append(blocked, t)
		} else {
			q.active[t] = true
			close(t.ready)
		}
	}
	q.pending = blocked
}
func (t *projectMutationTicket) wait(ctx context.Context) bool {
	if ctx.Err() != nil {
		t.release()
		return false
	}
	select {
	case <-ctx.Done():
		t.release()
		return false
	case <-t.ready:
		if ctx.Err() != nil {
			t.release()
			return false
		}
		return true
	}
}
func (t *projectMutationTicket) release() {
	t.once.Do(func() {
		q := t.q
		q.mu.Lock()
		defer q.mu.Unlock()
		delete(q.active, t)
		for i, p := range q.pending {
			if p == t {
				q.pending = append(q.pending[:i], q.pending[i+1:]...)
				break
			}
		}
		q.schedule()
	})
}

// An ancestor-aware project lease lets a shared writer own its project while
// main can keep talking/reading. Nested cwd changes cannot bypass that lease.
func (a *App) lockSubAgentProject(ctx context.Context, dir string) (func(), error) {
	root, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	if real, err := filepath.EvalSymlinks(root); err == nil {
		root = real
	}
	ticket := a.subAgentWrites.reserve(filepath.Clean(root))
	if !ticket.wait(ctx) {
		return nil, ctx.Err()
	}
	return ticket.release, nil
}
