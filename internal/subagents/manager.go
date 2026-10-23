// Package subagents runs independent, resumable child tasks and persists their
// completion mailbox. The caller owns provider selection and child tool policy.
package subagents

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"memo/internal/provider"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"sync"
	"time"
)

type State string

const (
	Queued      State = "queued"
	Running     State = "running"
	Awaiting    State = "awaiting_followup"
	Failed      State = "failed"
	Cancelled   State = "cancelled"
	Interrupted State = "interrupted"
	Closed      State = "closed"
)

type Event struct {
	Type     string    `json:"type"`
	Content  string    `json:"content"`
	ToolName string    `json:"tool_name,omitempty"`
	Result   string    `json:"result,omitempty"`
	Error    string    `json:"error,omitempty"`
	Args     any       `json:"args,omitempty"`
	At       time.Time `json:"at"`
}
type Job struct {
	ResultDelivery string `json:"result_delivery,omitempty"`
	DeliveryError  string `json:"delivery_error,omitempty"`
	Write          bool   `json:"write"`
	SystemPrompt   string `json:"system_prompt,omitempty"`
	ProviderName   string `json:"provider_name,omitempty"`
	Model          string `json:"model,omitempty"`
	Effort         string `json:"effort,omitempty"`

	Title          string             `json:"title"`
	ID             string             `json:"id"`
	ChatID         string             `json:"chat_id"`
	Prompt         string             `json:"prompt"`
	Skills         []string           `json:"skills,omitempty"`
	ProjectDir     string             `json:"project_dir,omitempty"`
	WorkDir        string             `json:"worktree,omitempty"`
	Branch         string             `json:"branch,omitempty"`
	BaseCommit     string             `json:"base_commit,omitempty"`
	Isolated       bool               `json:"isolated"`
	Merged         bool               `json:"merged"`
	State          State              `json:"status"`
	CompletedRound int                `json:"completed_round"`
	Round          int                `json:"round"`
	Messages       []provider.Message `json:"messages"`
	Events         []Event            `json:"events"`
	Result         string             `json:"output,omitempty"`
	Error          string             `json:"error,omitempty"`
	CreatedAt      time.Time          `json:"created_at"`
	UpdatedAt      time.Time          `json:"updated_at"`
}
type CreateRequest struct {
	Write        bool   `json:"write"`
	SystemPrompt string `json:"system_prompt,omitempty"`
	ProviderName string `json:"provider_name,omitempty"`
	Model        string `json:"model,omitempty"`
	Effort       string `json:"effort,omitempty"`

	Title      string   `json:"title"`
	ChatID     string   `json:"chat_id"`
	Prompt     string   `json:"prompt"`
	Skills     []string `json:"skills,omitempty"`
	ProjectDir string   `json:"project_dir,omitempty"`
	Isolated   bool     `json:"isolated"`
}
type RunResult struct {
	Output   string
	Messages []provider.Message
}
type Runner func(context.Context, Job, func(Event)) (RunResult, error)
type Completion struct {
	DeliveryError string `json:"delivery_error,omitempty"`
	Delivery      string `json:"delivery"`
	ID            uint64 `json:"id"`
	JobID         string `json:"job_id"`
	ChatID        string `json:"chat_id"`
	Round         int    `json:"round"`
	Result        string `json:"result"`
	Error         string `json:"error,omitempty"`
	State         State  `json:"status"`
}
type diskState struct {
	Jobs        map[string]*Job `json:"jobs"`
	Queue       []string        `json:"queue"`
	Completions []Completion    `json:"completions"`
	Sequence    uint64          `json:"sequence"`
}
type Manager struct {
	mu                 sync.Mutex
	gitMu              sync.Mutex
	dir                string
	limit              int
	runner             Runner
	data               diskState
	active             map[string]context.CancelFunc
	integrating        map[string]bool
	wg                 sync.WaitGroup
	started            bool
	stopping           bool
	persistErr         error
	lastSave           time.Time
	streamDirty        bool
	checkpointStop     chan struct{}
	completionNotifier func(Completion)
}

func Open(dir string, limit int, runner Runner) (*Manager, error) {
	if runner == nil {
		return nil, errors.New("subagent runner is required")
	}
	if limit < 1 || limit > 16 {
		return nil, errors.New("subagent concurrency must be positive")
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	m := &Manager{dir: dir, limit: limit, runner: runner, active: map[string]context.CancelFunc{}, integrating: map[string]bool{}, checkpointStop: make(chan struct{}), data: diskState{Jobs: map[string]*Job{}}}
	b, err := os.ReadFile(filepath.Join(dir, "state.json"))
	if err == nil {
		if err = json.Unmarshal(b, &m.data); err != nil {
			return nil, fmt.Errorf("read subagent state: %w", err)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	if m.data.Jobs == nil {
		m.data.Jobs = map[string]*Job{}
	}
	for i := range m.data.Completions {
		if m.data.Completions[i].Delivery == "inflight" {
			m.data.Completions[i].Delivery = "interrupted"
			m.data.Completions[i].DeliveryError = "Application stopped while the main agent was handling this result; explicit retry is required"
		}
		if m.data.Completions[i].Delivery == "" {
			m.data.Completions[i].Delivery = "pending"
		}
	}
	for _, j := range m.data.Jobs {
		if j.State == Cancelled && j.CompletedRound < j.Round {
			m.completeLocked(j)
		}
		if j.State == Running {
			j.State = Interrupted
			j.Error = "Application stopped while this task was running; explicit followup is required"
			m.completeLocked(j)
		}
	}
	if err = m.saveLocked(); err != nil {
		return nil, err
	}
	return m, nil
}
func clone[T any](v T) T { b, _ := json.Marshal(v); var out T; _ = json.Unmarshal(b, &out); return out }
func (m *Manager) saveLocked() error {
	b, err := json.Marshal(m.data)
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(m.dir, ".state-*")
	if err != nil {
		return err
	}
	name := f.Name()
	defer os.Remove(name)
	if err = f.Chmod(0600); err == nil {
		_, err = f.Write(b)
	}
	if err == nil {
		err = f.Sync()
	}
	cerr := f.Close()
	if err == nil {
		err = cerr
	}
	if err == nil {
		err = os.Rename(name, filepath.Join(m.dir, "state.json"))
	}
	// Windows supports flushing the state file, but not a directory handle.
	if err == nil && runtime.GOOS != "windows" {
		if d, e := os.Open(m.dir); e == nil {
			err = d.Sync()
			_ = d.Close()
		}
	}
	if err == nil {
		m.lastSave = time.Now()
		m.streamDirty = false
	}
	return err
}
func (m *Manager) ownedLocked(chat, id string) (*Job, error) {
	j := m.data.Jobs[id]
	if j == nil || j.ChatID != chat {
		return nil, errors.New("subagent task not found in this chat")
	}
	return j, nil
}
func (m *Manager) readyLocked() error {
	if m.stopping {
		return errors.New("subagent manager is stopping")
	}
	if m.persistErr != nil {
		return fmt.Errorf("subagent persistence failed: %w", m.persistErr)
	}
	return nil
}
func (m *Manager) Create(req CreateRequest) (Job, error) {
	if req.ChatID == "" || req.Prompt == "" {
		return Job{}, errors.New("chat and prompt are required")
	}
	var random [12]byte
	if _, err := rand.Read(random[:]); err != nil {
		return Job{}, err
	}
	now := time.Now().UTC()
	j := Job{ID: hex.EncodeToString(random[:]), ChatID: req.ChatID, Title: req.Title, Write: req.Write, SystemPrompt: req.SystemPrompt, ProviderName: req.ProviderName, Model: req.Model, Effort: req.Effort, Prompt: req.Prompt, Skills: append([]string(nil), req.Skills...), ProjectDir: req.ProjectDir, WorkDir: req.ProjectDir, Isolated: req.Isolated, State: Queued, Round: 1, Messages: []provider.Message{provider.TextMessage("user", req.Prompt)}, CreatedAt: now, UpdatedAt: now}
	if j.WorkDir != "" {
		resolved, err := filepath.Abs(j.WorkDir)
		if err != nil {
			return Job{}, err
		}
		if real, err := filepath.EvalSymlinks(resolved); err == nil {
			resolved = real
		}
		j.ProjectDir = resolved
		j.WorkDir = resolved
	}
	// Serialize workspace creation against merges, before publishing the task.
	m.gitMu.Lock()
	defer m.gitMu.Unlock()
	m.mu.Lock()
	readyErr := m.readyLocked()
	m.mu.Unlock()
	if readyErr != nil {
		return Job{}, readyErr
	}
	if req.Isolated {
		if err := createWorkspace(context.Background(), m.dir, &j); err != nil {
			return Job{}, err
		}
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.readyLocked(); err != nil {
		if j.Isolated {
			_ = discardUnusedWorkspace(context.Background(), j)
		}
		return Job{}, err
	}
	m.data.Jobs[j.ID] = &j
	m.data.Queue = append(m.data.Queue, j.ID)
	if err := m.saveLocked(); err != nil {
		delete(m.data.Jobs, j.ID)
		m.data.Queue = m.data.Queue[:len(m.data.Queue)-1]
		if j.Isolated {
			_ = discardUnusedWorkspace(context.Background(), j)
		}
		return Job{}, err
	}
	out := clone(j)
	m.scheduleLocked()
	return out, nil
}
func (m *Manager) Followup(chat, id, prompt string) (Job, error) {
	m.gitMu.Lock()
	defer m.gitMu.Unlock()
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.readyLocked(); err != nil {
		return Job{}, err
	}
	j, err := m.ownedLocked(chat, id)
	if err != nil {
		return Job{}, err
	}
	if prompt == "" {
		return Job{}, errors.New("followup prompt is required")
	}
	if m.active[id] != nil || j.State == Running || j.State == Queued || j.State == Closed || j.Merged {
		return Job{}, errors.New("task cannot accept a followup in its current state")
	}
	old := clone(*j)
	j.Round++
	j.Prompt = prompt
	j.Messages = append(j.Messages, provider.TextMessage("user", prompt))
	j.State = Queued
	j.Result = ""
	j.Error = ""
	j.UpdatedAt = time.Now().UTC()
	m.data.Queue = append(m.data.Queue, id)
	if err = m.saveLocked(); err != nil {
		*j = old
		m.data.Queue = m.data.Queue[:len(m.data.Queue)-1]
		return Job{}, err
	}
	out := clone(*j)
	m.scheduleLocked()
	return out, nil
}
func (m *Manager) scheduleLocked() {
	for m.started && !m.stopping && m.persistErr == nil && len(m.active) < m.limit && len(m.data.Queue) > 0 {
		index := -1
		for i, candidate := range m.data.Queue {
			j := m.data.Jobs[candidate]
			blocked := false
			if j != nil && j.Write && !j.Isolated {
				for repo := range m.integrating {
					if pathsOverlap(repo, j.WorkDir) {
						blocked = true
						break
					}
				}
				for activeID := range m.active {
					active := m.data.Jobs[activeID]
					if active.Write && !active.Isolated && pathsOverlap(active.WorkDir, j.WorkDir) {
						blocked = true
						break
					}
				}
			}
			if !blocked {
				index = i
				break
			}
		}
		if index < 0 {
			return
		}
		id := m.data.Queue[index]
		m.data.Queue = append(m.data.Queue[:index], m.data.Queue[index+1:]...)
		j := m.data.Jobs[id]
		if j == nil || j.State != Queued {
			continue
		}
		j.State = Running
		j.UpdatedAt = time.Now().UTC()
		if err := m.saveLocked(); err != nil {
			j.State = Queued
			m.data.Queue = append([]string{id}, m.data.Queue...)
			m.persistErr = err
			return
		}
		ctx, cancel := context.WithCancel(context.Background())
		m.active[id] = cancel
		m.wg.Add(1)
		go m.run(ctx, clone(*j))
	}
}
func (m *Manager) run(ctx context.Context, snapshot Job) {
	defer m.wg.Done()
	var result RunResult
	var runErr error
	func() {
		defer func() {
			if r := recover(); r != nil {
				runErr = fmt.Errorf("subagent runner panic: %v", r)
			}
		}()
		result, runErr = m.runner(ctx, snapshot, func(e Event) {
			m.mu.Lock()
			defer m.mu.Unlock()
			j := m.data.Jobs[snapshot.ID]
			if e.At.IsZero() {
				e.At = time.Now().UTC()
			}
			streamDelta := e.Type == "output" || e.Type == "thinking" || e.Type == "narration_delta"
			if streamDelta && len(j.Events) > 0 && j.Events[len(j.Events)-1].Type == e.Type {
				last := &j.Events[len(j.Events)-1]
				last.Content += e.Content
				last.At = e.At
			} else {
				j.Events = append(j.Events, e)
			}
			j.UpdatedAt = e.At
			// Queue/state transitions and tool events are durable immediately. Tiny
			// streamed deltas share one checkpoint per 250ms across all children; run
			// finalization always forces a final atomic save before notification.
			if streamDelta && !m.stopping && time.Since(m.lastSave) < 250*time.Millisecond {
				m.streamDirty = true
				return
			}
			if err := m.saveLocked(); err != nil {
				m.persistErr = err
				if cancel := m.active[j.ID]; cancel != nil {
					cancel()
				}
			}
		})
	}()
	m.mu.Lock()
	defer m.mu.Unlock()
	j := m.data.Jobs[snapshot.ID]
	if cancel := m.active[j.ID]; cancel != nil {
		cancel()
	}
	delete(m.active, j.ID)
	if len(result.Messages) > 0 {
		j.Messages = result.Messages
	}
	j.Result = result.Output
	j.Error = ""
	if runErr != nil {
		j.Error = runErr.Error()
	}
	if j.State == Cancelled {
	} else if m.stopping {
		j.State = Interrupted
		if j.Error == "" {
			j.Error = "Application stopped while task was running"
		}
	} else if runErr != nil {
		j.State = Failed
	} else {
		j.State = Awaiting
	}
	j.UpdatedAt = time.Now().UTC()
	completion := m.completeLocked(j)
	if err := m.saveLocked(); err != nil {
		m.persistErr = err
	} else if completion != nil {
		m.notifyLocked(*completion)
	}
	m.scheduleLocked()
}
func (m *Manager) completeLocked(j *Job) *Completion {
	if j.CompletedRound >= j.Round {
		return nil
	}
	j.CompletedRound = j.Round
	m.data.Sequence++
	m.data.Completions = append(m.data.Completions, Completion{Delivery: "pending", ID: m.data.Sequence, JobID: j.ID, ChatID: j.ChatID, Round: j.Round, Result: j.Result, Error: j.Error, State: j.State})
	return &m.data.Completions[len(m.data.Completions)-1]
}
func (m *Manager) Get(chat, id string) (Job, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	j, err := m.ownedLocked(chat, id)
	if err != nil {
		return Job{}, err
	}
	return m.jobViewLocked(j), nil
}
func (m *Manager) List(chat string) []Job {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := []Job{}
	for _, j := range m.data.Jobs {
		if j.ChatID == chat {
			out = append(out, m.jobViewLocked(j))
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.Before(out[j].CreatedAt) })
	return out
}
func (m *Manager) PeekCompletion(chat string) (Completion, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, c := range m.data.Completions {
		if c.ChatID == chat {
			return c, true
		}
	}
	return Completion{}, false
}
func (m *Manager) AckCompletion(chat string, id uint64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i, c := range m.data.Completions {
		if c.ChatID != chat {
			continue
		}
		if c.ID != id {
			return errors.New("completion must be acknowledged in FIFO order")
		}
		if c.ID == id && c.ChatID == chat {
			old := m.data.Completions
			m.data.Completions = append(append([]Completion(nil), old[:i]...), old[i+1:]...)
			if err := m.saveLocked(); err != nil {
				m.data.Completions = old
				return err
			}
			return nil
		}
	}
	return errors.New("completion not found in this chat")
}
func (m *Manager) SetLimit(limit int) error {
	if limit < 1 || limit > 16 {
		return errors.New("concurrency must be between 1 and 16")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.readyLocked(); err != nil {
		return err
	}
	m.limit = limit
	m.scheduleLocked()
	return nil
}
func (m *Manager) Cancel(chat, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	j, err := m.ownedLocked(chat, id)
	if err != nil {
		return err
	}
	if j.State != Running && j.State != Queued {
		return errors.New("task is not active")
	}
	old := clone(*j)
	j.State = Cancelled
	j.Error = "Task cancelled"
	j.UpdatedAt = time.Now().UTC()
	wasRunning := m.active[id] != nil
	var completion *Completion
	if !wasRunning {
		completion = m.completeLocked(j)
	}
	if err = m.saveLocked(); err != nil {
		*j = old
		if !wasRunning {
			m.data.Completions = m.data.Completions[:len(m.data.Completions)-1]
			m.data.Sequence--
		}
		return err
	}
	if completion != nil {
		m.notifyLocked(*completion)
	}
	if cancel := m.active[id]; cancel != nil {
		cancel()
	}
	m.scheduleLocked()
	return nil
}
func (m *Manager) Shutdown(ctx context.Context) error {
	m.mu.Lock()
	if !m.stopping {
		m.stopping = true
		close(m.checkpointStop)
	}
	if m.streamDirty {
		if err := m.saveLocked(); err != nil {
			m.persistErr = err
		}
	}
	for _, cancel := range m.active {
		cancel()
	}
	m.mu.Unlock()
	done := make(chan struct{})
	go func() { m.wg.Wait(); close(done) }()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-done:
		m.mu.Lock()
		defer m.mu.Unlock()
		return m.persistErr
	}
}

// CloseJob removes transient task data after its result has been accepted. An
// isolated task must first be merged; unmerged work is never silently discarded.
func (m *Manager) CloseJob(chat, id string) error {
	m.gitMu.Lock()
	defer m.gitMu.Unlock()
	m.mu.Lock()
	defer m.mu.Unlock()
	j, err := m.ownedLocked(chat, id)
	if err != nil {
		return err
	}
	if m.active[id] != nil || j.State == Running || j.State == Queued {
		return errors.New("active task cannot be closed")
	}
	if j.Isolated && !j.Merged {
		return errors.New("merge isolated task before closing it")
	}
	if j.Isolated {
		if err := cleanupWorkspace(context.Background(), *j); err != nil {
			return err
		}
	}
	old := clone(*j)
	j.State = Closed
	j.Messages = nil
	j.Events = nil
	j.Result = ""
	j.Prompt = ""
	j.SystemPrompt = ""
	j.Skills = nil
	j.Error = ""
	if err = m.saveLocked(); err != nil {
		*j = old
	}
	return err
}

// Start enables execution after the caller has initialized runner dependencies.
func (m *Manager) Start() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.readyLocked(); err != nil {
		return err
	}
	if !m.started {
		m.started = true
		m.wg.Add(1)
		go m.checkpointStream()
	}
	m.scheduleLocked()
	return m.persistErr
}
func (m *Manager) PendingChats() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	seen := map[string]bool{}
	out := []string{}
	for _, c := range m.data.Completions {
		if !seen[c.ChatID] {
			seen[c.ChatID] = true
			out = append(out, c.ChatID)
		}
	}
	return out
}

func (m *Manager) ClaimCompletion(chat string, id uint64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i := range m.data.Completions {
		c := &m.data.Completions[i]
		if c.ChatID != chat {
			continue
		}
		if c.ID != id {
			return errors.New("completion must be claimed in FIFO order")
		}
		if c.Delivery != "pending" {
			return errors.New("completion is already claimed or interrupted")
		}
		c.Delivery = "inflight"
		if err := m.saveLocked(); err != nil {
			c.Delivery = "pending"
			return err
		}
		return nil
	}
	return errors.New("completion not found")
}
func (m *Manager) RetryCompletion(chat, jobID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, err := m.ownedLocked(chat, jobID); err != nil {
		return err
	}
	for i := range m.data.Completions {
		c := &m.data.Completions[i]
		if c.ChatID == chat && c.JobID == jobID && c.Delivery == "interrupted" {
			oldError := c.DeliveryError
			c.Delivery = "pending"
			c.DeliveryError = ""
			if err := m.saveLocked(); err != nil {
				c.Delivery = "interrupted"
				c.DeliveryError = oldError
				return err
			}
			m.notifyLocked(*c)
			return nil
		}
	}
	return errors.New("interrupted completion not found")
}

// jobViewLocked derives delivery status without duplicating mutable mailbox
// state on disk. Closing a task never acknowledges or removes its completion.
func (m *Manager) jobViewLocked(j *Job) Job {
	out := clone(*j)
	for _, c := range m.data.Completions {
		if c.JobID == j.ID && c.ChatID == j.ChatID {
			out.ResultDelivery = c.Delivery
			out.DeliveryError = c.DeliveryError
			break
		}
	}
	return out
}
func (m *Manager) Completions(chat string) []Completion {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := []Completion{}
	for _, c := range m.data.Completions {
		if c.ChatID == chat {
			out = append(out, c)
		}
	}
	return out
}
func (m *Manager) MarkDeliveryInterrupted(chat string, id uint64, reason string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i := range m.data.Completions {
		c := &m.data.Completions[i]
		if c.ChatID != chat {
			continue
		}
		if c.ID != id {
			return errors.New("completion interruption must follow FIFO order")
		}
		if c.Delivery != "inflight" {
			return errors.New("completion is not being delivered")
		}
		old := *c
		c.Delivery = "interrupted"
		c.DeliveryError = reason
		if err := m.saveLocked(); err != nil {
			*c = old
			return err
		}
		return nil
	}
	return errors.New("completion not found")
}

func (m *Manager) checkpointStream() {
	defer m.wg.Done()
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-m.checkpointStop:
			return
		case <-ticker.C:
			m.mu.Lock()
			if m.streamDirty && !m.stopping {
				if err := m.saveLocked(); err != nil {
					m.persistErr = err
					for _, cancel := range m.active {
						cancel()
					}
				}
			}
			m.mu.Unlock()
		}
	}
}

// SetCompletionNotifier installs an arrival hook before Start. The callback is
// synchronous, under the manager mutex, and MUST NOT call manager methods. It
// should only reserve the parent's FIFO ticket and launch asynchronous delivery.
// Notifications happen only after durable saves; existing recovery events must
// be obtained separately through Completions during application startup.
func (m *Manager) SetCompletionNotifier(notify func(Completion)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.completionNotifier = notify
}
func (m *Manager) notifyLocked(c Completion) {
	if m.completionNotifier == nil {
		return
	}
	defer func() {
		if r := recover(); r != nil {
			for i := range m.data.Completions {
				event := &m.data.Completions[i]
				if event.ID == c.ID {
					event.Delivery = "interrupted"
					event.DeliveryError = fmt.Sprintf("Completion notification interrupted: %v", r)
					break
				}
			}
			if err := m.saveLocked(); err != nil {
				m.persistErr = err
			}
		}
	}()
	m.completionNotifier(c)
}
