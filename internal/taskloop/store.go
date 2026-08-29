package taskloop

import (
	"encoding/json"
	"fmt"
	"memo/internal/fileutil"
	"memo/internal/logx"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
)

const maxRoundsPerItem = 5

type TaskItem struct {
	ID     string `json:"id"`
	Text   string `json:"text"`
	Status string `json:"status"` // pending | running | done | stuck
	Note   string `json:"note"`
	Rounds int    `json:"rounds"`
	// Line is the 1-based source line in TaskList.TaskMdPath this item came
	// from (0 when the list wasn't seeded from a file), so completion can
	// mirror "[ ]" -> "[x]" back into that exact line.
	Line       int    `json:"line,omitempty"`
	StartedAt  string `json:"started_at,omitempty"`
	FinishedAt string `json:"finished_at,omitempty"`
}

type TaskList struct {
	ID     string     `json:"id"`
	ChatID string     `json:"chat_id"`
	Title  string     `json:"title"`
	Items  []TaskItem `json:"items"`
	// Status is one of the taskListStatus* values below. "running" is legacy
	// (pre-v4.4.0) — new writes use "planning"/"executing"; loadAll maps a
	// leftover "running" to "paused" on restart.
	Status string `json:"status"`
	// NotifyLevel is parsed from the "# bildirim:" header of the source
	// Task.md and drives how chatty the NotifyBus is for this list.
	NotifyLevel NotifyLevel `json:"notify_level,omitempty"`
	// TaskMdPath is the absolute path of the Task.md this list was seeded
	// from, so item completion can mirror "[ ]" -> "[x]" back into it. Empty
	// for lists created directly (Tasks screen / REST / REPL).
	TaskMdPath string `json:"task_md_path,omitempty"`
	CreatedAt  string `json:"created_at"`
	UpdatedAt  string `json:"updated_at"`
}

// Task list lifecycle states.
const (
	taskListIdle         = "idle"
	taskListPlanning     = "planning"
	taskListExecuting    = "executing"
	taskListRunning      = "running" // legacy, still accepted for reads/writes during transition
	taskListWaitingLimit = "waiting-limit"
	taskListWaitingUser  = "waiting-user"
	taskListPaused       = "paused"
	taskListDone         = "done"
	taskListFailed       = "failed"
	taskListCancelled    = "cancelled"
)

func validTaskListStatus(s string) bool {
	switch s {
	case taskListIdle, taskListPlanning, taskListExecuting, taskListRunning,
		taskListWaitingLimit, taskListWaitingUser, taskListPaused,
		taskListDone, taskListFailed, taskListCancelled:
		return true
	default:
		return false
	}
}

type TaskListInfo struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Status    string `json:"status"`
	ItemCount int    `json:"item_count"`
	DoneCount int    `json:"done_count"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

type Store struct {
	dir  string
	mu   sync.RWMutex
	list map[string]*TaskList
}

func NewStore(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("taskloop: mkdir: %w", err)
	}
	s := &Store{
		dir:  dir,
		list: make(map[string]*TaskList),
	}
	if err := s.loadAll(); err != nil {
		return nil, err
	}
	return s, nil
}

// Create makes a task list with the default notification level and no source
// Task.md. Kept for the Tasks screen / REST / REPL callers that pass a plain
// item list.
func (s *Store) Create(chatID, title string, items []string) (*TaskList, error) {
	return s.CreateWithMeta(chatID, title, items, NotifyImportant, "")
}

// CreateWithMeta makes a task list, recording the notification level and the
// source Task.md path so the engine can mirror checkbox state back into the
// file. An empty notify defaults to NotifyImportant.
func (s *Store) CreateWithMeta(chatID, title string, items []string, notify NotifyLevel, taskMdPath string) (*TaskList, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if notify == "" {
		notify = NotifyImportant
	}
	now := time.Now().Format("2006-01-02 15:04")
	taskItems := make([]TaskItem, len(items))
	for i, text := range items {
		taskItems[i] = TaskItem{
			ID:     uuid.New().String(),
			Text:   text,
			Status: "pending",
		}
	}
	tl := &TaskList{
		ID:          uuid.New().String(),
		ChatID:      chatID,
		Title:       title,
		Items:       taskItems,
		Status:      taskListIdle,
		NotifyLevel: notify,
		TaskMdPath:  taskMdPath,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	s.list[tl.ID] = tl
	if err := s.save(tl); err != nil {
		return nil, err
	}
	return tl, nil
}

func (s *Store) Get(id string) (*TaskList, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	tl, ok := s.list[id]
	if !ok {
		return nil, fmt.Errorf("tasklist %s not found", id)
	}
	return s.clone(tl), nil
}

func (s *Store) List() []TaskListInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make([]TaskListInfo, 0, len(s.list))
	for _, tl := range s.list {
		done := 0
		for _, item := range tl.Items {
			if item.Status == "done" {
				done++
			}
		}
		out = append(out, TaskListInfo{
			ID:        tl.ID,
			Title:     tl.Title,
			Status:    tl.Status,
			ItemCount: len(tl.Items),
			DoneCount: done,
			CreatedAt: tl.CreatedAt,
			UpdatedAt: tl.UpdatedAt,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].UpdatedAt > out[j].UpdatedAt
	})
	return out
}

func (s *Store) SetStatus(id, status string) error {
	if !validTaskListStatus(status) {
		return fmt.Errorf("taskloop: invalid task list status %q", status)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	tl, ok := s.list[id]
	if !ok {
		return fmt.Errorf("tasklist %s not found", id)
	}
	tl.Status = status
	tl.UpdatedAt = time.Now().Format("2006-01-02 15:04")
	return s.save(tl)
}

// SetItemLine records which Task.md source line an item maps to (used when a
// list is seeded from a file so completion can flip its checkbox).
func (s *Store) SetItemLine(listID, itemID string, line int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	tl, ok := s.list[listID]
	if !ok {
		return fmt.Errorf("tasklist %s not found", listID)
	}
	for i := range tl.Items {
		if tl.Items[i].ID == itemID {
			tl.Items[i].Line = line
			return s.save(tl)
		}
	}
	return fmt.Errorf("item %s not found in list %s", itemID, listID)
}

func (s *Store) SetItemRunning(listID, itemID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	tl, ok := s.list[listID]
	if !ok {
		return fmt.Errorf("tasklist %s not found", listID)
	}
	for i := range tl.Items {
		if tl.Items[i].ID == itemID {
			tl.Items[i].Status = "running"
			tl.Items[i].StartedAt = time.Now().Format("2006-01-02 15:04")
			tl.UpdatedAt = tl.Items[i].StartedAt
			return s.save(tl)
		}
	}
	return fmt.Errorf("item %s not found in list %s", itemID, listID)
}

// ResetItemPending puts an interrupted item back to "pending" so a resumed
// run retries it instead of skipping it forever.
func (s *Store) ResetItemPending(listID, itemID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	tl, ok := s.list[listID]
	if !ok {
		return fmt.Errorf("tasklist %s not found", listID)
	}
	for i := range tl.Items {
		if tl.Items[i].ID == itemID {
			tl.Items[i].Status = "pending"
			tl.Items[i].StartedAt = ""
			tl.UpdatedAt = time.Now().Format("2006-01-02 15:04")
			return s.save(tl)
		}
	}
	return fmt.Errorf("item %s not found in list %s", itemID, listID)
}

func (s *Store) SetItemDone(listID, itemID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	tl, ok := s.list[listID]
	if !ok {
		return fmt.Errorf("tasklist %s not found", listID)
	}
	for i := range tl.Items {
		if tl.Items[i].ID == itemID {
			tl.Items[i].Status = "done"
			tl.Items[i].FinishedAt = time.Now().Format("2006-01-02 15:04")
			tl.UpdatedAt = tl.Items[i].FinishedAt
			return s.save(tl)
		}
	}
	return fmt.Errorf("item %s not found in list %s", itemID, listID)
}

func (s *Store) SetItemStuck(listID, itemID, note string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	tl, ok := s.list[listID]
	if !ok {
		return fmt.Errorf("tasklist %s not found", listID)
	}
	for i := range tl.Items {
		if tl.Items[i].ID == itemID {
			tl.Items[i].Status = "stuck"
			tl.Items[i].Note = note
			tl.Items[i].FinishedAt = time.Now().Format("2006-01-02 15:04")
			tl.UpdatedAt = tl.Items[i].FinishedAt
			return s.save(tl)
		}
	}
	return fmt.Errorf("item %s not found in list %s", itemID, listID)
}

func (s *Store) IncrementRounds(listID, itemID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	tl, ok := s.list[listID]
	if !ok {
		return fmt.Errorf("tasklist %s not found", listID)
	}
	for i := range tl.Items {
		if tl.Items[i].ID == itemID {
			tl.Items[i].Rounds++
			tl.UpdatedAt = time.Now().Format("2006-01-02 15:04")
			return s.save(tl)
		}
	}
	return fmt.Errorf("item %s not found in list %s", itemID, listID)
}

func (s *Store) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.list[id]; !ok {
		return fmt.Errorf("tasklist %s not found", id)
	}
	delete(s.list, id)
	if err := os.Remove(filepath.Join(s.dir, id+".json")); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func (s *Store) save(tl *TaskList) error {
	data, err := json.MarshalIndent(tl, "", "  ")
	if err != nil {
		return err
	}
	path := filepath.Join(s.dir, tl.ID+".json")
	if err := fileutil.AtomicWrite(path, data, 0600); err != nil {
		return fmt.Errorf("taskloop: save %s: %w", tl.ID, err)
	}
	return nil
}

func (s *Store) loadAll() error {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	for _, e := range entries {
		if filepath.Ext(e.Name()) != ".json" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(s.dir, e.Name()))
		if err != nil {
			logx.Printf("taskloop: read %s: %v", e.Name(), err)
			continue
		}
		var tl TaskList
		if err := json.Unmarshal(data, &tl); err != nil {
			logx.Printf("taskloop: decode %s: %v", e.Name(), err)
			continue
		}
		// A list can be left in an active phase (planning/executing, or the
		// legacy "running") if the process was killed mid-loop. No Engine
		// goroutine survives a restart to finish it, so downgrade to "paused"
		// and reset any half-run item. "waiting-limit" and "waiting-user" are
		// deliberately kept: the former is re-armed by the retry scheduler on
		// startup, the latter still needs the user to act.
		switch tl.Status {
		case taskListRunning, taskListPlanning, taskListExecuting:
			tl.Status = taskListPaused
			for i := range tl.Items {
				if tl.Items[i].Status == "running" {
					tl.Items[i].Status = "pending"
					tl.Items[i].StartedAt = ""
				}
			}
		}
		s.list[tl.ID] = &tl
	}
	return nil
}

func (s *Store) clone(tl *TaskList) *TaskList {
	c := *tl
	c.Items = make([]TaskItem, len(tl.Items))
	copy(c.Items, tl.Items)
	return &c
}
