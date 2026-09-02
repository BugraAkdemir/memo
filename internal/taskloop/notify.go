package taskloop

import (
	"context"
	"sync"
)

// Notification is one task-loop event routed to the user's channels.
type Notification struct {
	ListID    string
	ListTitle string
	Event     string // canonical event name, e.g. "started", "finished", "waiting_limit"
	Detail    string
	// Body, when set, is the exact text to deliver (e.g. a model-written
	// completion report) — the senders use it verbatim instead of the
	// short built-in one-liner.
	Body string
}

// Sender delivers notifications to one channel (app event bus, Telegram,
// WhatsApp, ...). A failing Sender must not stop the others.
type Sender interface {
	SendNotification(ctx context.Context, n Notification) error
}

// NotifyBus fans notifications out to every registered Sender, filtered by the
// per-list verbosity level parsed from the Task.md "# bildirim:" header.
type NotifyBus struct {
	mu      sync.RWMutex
	senders []Sender
	levels  map[string]NotifyLevel
}

func NewNotifyBus() *NotifyBus {
	return &NotifyBus{levels: make(map[string]NotifyLevel)}
}

func (b *NotifyBus) AddSender(s Sender) {
	if s == nil {
		return
	}
	b.mu.Lock()
	b.senders = append(b.senders, s)
	b.mu.Unlock()
}

// SetLevel records a list's verbosity. Unknown/empty defaults to
// NotifyImportant at filter time.
func (b *NotifyBus) SetLevel(listID string, l NotifyLevel) {
	b.mu.Lock()
	b.levels[listID] = l
	b.mu.Unlock()
}

func (b *NotifyBus) levelFor(listID string) NotifyLevel {
	b.mu.RLock()
	l := b.levels[listID]
	b.mu.RUnlock()
	if !validNotifyLevel(string(l)) {
		return NotifyImportant
	}
	return l
}

// eventTier: 0 = always (sadece-bitince+), 1 = önemli+, 2 = her-şey only.
func eventTier(event string) int {
	switch event {
	case "finished", "failed":
		return 0
	case "started", "item_stuck", "waiting_limit", "waiting_user", "waiting_retry",
		"provider_switched", "config_changed":
		return 1
	default: // item_started, item_done, subagent_spawned, ...
		return 2
	}
}

// ShouldNotify reports whether an event of this tier passes a list's level.
func (b *NotifyBus) ShouldNotify(listID, event string) bool {
	switch b.levelFor(listID) {
	case NotifyOnlyDone:
		return eventTier(event) == 0
	case NotifyEverything:
		return true
	default: // NotifyImportant
		return eventTier(event) <= 1
	}
}

// Notify fans n out to every Sender if it passes the list's level filter.
// Sender errors are ignored (best-effort delivery).
func (b *NotifyBus) Notify(ctx context.Context, n Notification) {
	if !b.ShouldNotify(n.ListID, n.Event) {
		return
	}
	b.mu.RLock()
	senders := append([]Sender(nil), b.senders...)
	b.mu.RUnlock()
	for _, s := range senders {
		_ = s.SendNotification(ctx, n)
	}
}
