package mail

import (
	"context"
	"encoding/json"
	"fmt"
	"memo/internal/autonomy"
	netmail "net/mail"
	"strings"
	"sync"
	"time"
)

// WatcherConfig tunes the background email polling and triage loop.
type WatcherConfig struct {
	Enabled         bool
	PollInterval    time.Duration
	MaxScanPerPoll  int
	MaxAutonomyAge  time.Duration
	AutoIndexMemory bool
}

// WatcherStats tracks cumulative triage metrics for UI display and morning briefings.
type WatcherStats struct {
	TotalEvaluated     int       `json:"total_evaluated"`
	DroppedMarketing   int       `json:"dropped_marketing"`
	IndexedInRAG       int       `json:"indexed_in_rag"`
	PassedToAutonomy   int       `json:"passed_to_autonomy"`
	CalendarExtracted  int       `json:"calendar_extracted"`
	SecurityBlocked    int       `json:"security_blocked"`
	LastSecurityReason string    `json:"last_security_reason,omitempty"`
	ColdStarts         int       `json:"cold_starts"`
	LastSyncAt         time.Time `json:"last_sync_at,omitempty"`
}

// MemoryIndexFunc indexes a non-bulk email into Memo's RAG vector store.
type MemoryIndexFunc func(ctx context.Context, account Account, detail *MessageDetail, decision TriageDecision) error

// MailEventHandler receives every newly arrived email alongside its Gatekeeper TriageDecision.
type MailEventHandler func(ctx context.Context, account Account, detail *MessageDetail, decision TriageDecision)

// Watcher polls connected email accounts in the background, enforces the
// High-Water Mark cold-start flood guard, runs the 3-layer Gatekeeper, and
// dispatches clean messages to RAG memory and the Digital Twin engine.
type Watcher struct {
	manager    *Manager
	gatekeeper *Gatekeeper

	mu      sync.RWMutex
	cfg     WatcherConfig
	stats   WatcherStats
	indexFn MemoryIndexFunc
	onEvent MailEventHandler
	guard   func(context.Context, Account, *MessageDetail) autonomy.Verdict

	triggerCh chan struct{}
	pollMu    sync.Mutex
}

// NewWatcher creates a new background mail Watcher.
func NewWatcher(manager *Manager, gatekeeper *Gatekeeper, cfg WatcherConfig) *Watcher {
	if cfg.PollInterval <= 0 {
		cfg.PollInterval = 60 * time.Second
	}
	if cfg.MaxScanPerPoll <= 0 {
		cfg.MaxScanPerPoll = 25
	}
	if cfg.MaxAutonomyAge <= 0 {
		cfg.MaxAutonomyAge = 6 * time.Hour
	}
	if gatekeeper == nil {
		gatekeeper = NewGatekeeper(nil)
	}
	return &Watcher{
		manager:    manager,
		gatekeeper: gatekeeper,
		cfg:        cfg,
		triggerCh:  make(chan struct{}, 1),
	}
}

// SetMemoryIndexer registers the callback that saves relevant emails into RAG memory.
func (w *Watcher) SetMemoryIndexer(fn MemoryIndexFunc) {
	w.mu.Lock()
	w.indexFn = fn
	w.mu.Unlock()
}

// SetEventHandler registers the callback invoked for each evaluated new message.
func (w *Watcher) SetEventHandler(fn MailEventHandler) {
	w.mu.Lock()
	w.onEvent = fn
	w.mu.Unlock()
}

// SetSecurityGuard installs the optional semantic review/OTP observer. A
// deterministic guard still runs when no app-level callback is installed.
func (w *Watcher) SetSecurityGuard(fn func(context.Context, Account, *MessageDetail) autonomy.Verdict) {
	w.mu.Lock()
	w.guard = fn
	w.mu.Unlock()
}

// RecordSecurityBlocked records codes only, never the mail/secret itself.
func (w *Watcher) RecordSecurityBlocked(code string) {
	w.mu.Lock()
	w.stats.SecurityBlocked++
	w.stats.LastSecurityReason = code
	w.mu.Unlock()
}

// UpdateConfig updates the live Watcher configuration.
func (w *Watcher) UpdateConfig(cfg WatcherConfig) {
	if cfg.PollInterval <= 0 {
		cfg.PollInterval = 60 * time.Second
	}
	if cfg.MaxScanPerPoll <= 0 {
		cfg.MaxScanPerPoll = 25
	}
	if cfg.MaxAutonomyAge <= 0 {
		cfg.MaxAutonomyAge = 6 * time.Hour
	}
	w.mu.Lock()
	w.cfg = cfg
	w.mu.Unlock()
}

// Stats returns a snapshot of the Watcher's cumulative triage metrics.
func (w *Watcher) Stats() WatcherStats {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.stats
}

// RecordCalendarAdded counts committed events, including plain-text intent extraction.
func (w *Watcher) RecordCalendarAdded() {
	w.mu.Lock()
	w.stats.CalendarExtracted++
	w.mu.Unlock()
}

// TriggerNow wakes the background poll loop immediately (non-blocking).
func (w *Watcher) TriggerNow() {
	select {
	case w.triggerCh <- struct{}{}:
	default:
	}
}

// Start runs the background polling loop until ctx is cancelled.
func (w *Watcher) Start(ctx context.Context) {
	// Initial sync pass shortly after startup
	w.mu.RLock()
	enabled := w.cfg.Enabled
	interval := w.cfg.PollInterval
	w.mu.RUnlock()

	if enabled {
		_ = w.SyncOnce(ctx)
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-w.triggerCh:
			_ = w.SyncOnce(ctx)
		case <-ticker.C:
			w.mu.RLock()
			en := w.cfg.Enabled
			newInterval := w.cfg.PollInterval
			w.mu.RUnlock()
			if newInterval != interval && newInterval > 0 {
				interval = newInterval
				ticker.Reset(interval)
			}
			if en {
				_ = w.SyncOnce(ctx)
			}
		}
	}
}

// SyncOnce polls all connected accounts once and processes any messages with
// UID > LastSeenUID through the 3-layer Gatekeeper.
func (w *Watcher) SyncOnce(ctx context.Context) error {
	w.pollMu.Lock()
	defer w.pollMu.Unlock()

	if w.manager == nil {
		return nil
	}

	w.mu.RLock()
	cfg := w.cfg
	indexFn := w.indexFn
	onEvent := w.onEvent
	guard := w.guard
	w.mu.RUnlock()

	accounts := w.manager.ListAccounts()
	if len(accounts) == 0 {
		return nil
	}

	var firstErr error
	for _, acct := range accounts {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		_, coldStart, newMsgs, err := w.manager.SyncAccountIncremental(ctx, acct.ID, cfg.MaxScanPerPoll)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}

		w.mu.Lock()
		w.stats.LastSyncAt = time.Now()
		if coldStart {
			w.stats.ColdStarts++
		}
		w.mu.Unlock()

		if coldStart || len(newMsgs) == 0 {
			continue
		}

		for _, detail := range newMsgs {
			if detail == nil {
				continue
			}
			text, _ := FormatMailForMemory(acct, detail, TriageDecision{})
			verdict := autonomy.DetectPromptInjection(ctx, text, nil)
			if guard != nil {
				// The app also observes parsed OTPs here, including in denied
				// mail. Its review can never override a deterministic block.
				reviewed := guard(ctx, acct, detail)
				if !verdict.Blocked {
					verdict = reviewed
				}
			}
			if verdict.Blocked {
				w.mu.Lock()
				w.stats.TotalEvaluated++
				w.mu.Unlock()
				w.RecordSecurityBlocked(verdict.Code)
				if onEvent != nil {
					onEvent(ctx, acct, detail, TriageDecision{SecurityBlocked: true, SecurityReason: verdict.Code, DropFromRAG: true})
				}
				continue
			}
			decision := w.gatekeeper.Evaluate(ctx, detail)
			// Protocol/group gates cannot be bypassed by a VIP classification.
			recipients, _ := mailRecipients(detail.To, detail.Cc)
			policy := autonomy.AutoReplyPolicy(autonomy.Envelope{Account: acct.ID, Sender: detail.From, ThreadID: detail.InReplyTo, Headers: detail.Headers, RecipientCount: recipients})
			if policy.Blocked {
				decision.AllowAutoReply = false
			}

			// Backfill Age Guard: never allow autonomous replies to messages older
			// than MaxAutonomyAge (default 6 hours), even if UID > LastSeenUID
			// (e.g. user moved an old email into INBOX).
			if !detail.Timestamp.IsZero() && cfg.MaxAutonomyAge > 0 {
				if time.Since(detail.Timestamp) > cfg.MaxAutonomyAge {
					decision.AllowAutoReply = false
				}
			}

			w.mu.Lock()
			w.stats.TotalEvaluated++
			if decision.DropFromRAG {
				w.stats.DroppedMarketing++
			}
			if decision.AllowAutoReply {
				w.stats.PassedToAutonomy++
			}
			w.mu.Unlock()

			if !decision.DropFromRAG && decision.IndexInRAG && cfg.AutoIndexMemory && indexFn != nil {
				if err := indexFn(ctx, acct, detail, decision); err == nil {
					w.mu.Lock()
					w.stats.IndexedInRAG++
					w.mu.Unlock()
				}
			}

			if onEvent != nil {
				onEvent(ctx, acct, detail, decision)
			}
		}
	}

	return firstErr
}

// FormatMailForMemory keeps the entire mail on the user side of memory.
// The second return value is empty: mail content is not an assistant reply.
func FormatMailForMemory(account Account, detail *MessageDetail, decision TriageDecision) (userMsg, assistantMsg string) {
	body := strings.TrimSpace(detail.TextBody)
	if body == "" {
		body = stripHTMLTags(detail.HTMLBody)
	}
	dateStr := detail.Date
	if !detail.Timestamp.IsZero() {
		dateStr = detail.Timestamp.Format("2006-01-02 15:04")
	}
	userMsg = fmt.Sprintf("[Mail | Account: %s | Account-ID: %s | Folder: INBOX | UID: %d | Message-ID: %s | From: %s | Subject: %s | Date: %s | Category: %s]",
		account.Email, account.ID, detail.UID, detail.MessageID, detail.From, detail.Subject, dateStr, decision.Category)
	// Deliberately omit OTPCode: the Gatekeeper keeps OTP messages out of RAG.
	structured := StructuredExtraction{CalendarInvites: detail.Structured.CalendarInvites, Flights: detail.Structured.Flights, Invoices: detail.Structured.Invoices}
	if len(structured.CalendarInvites)+len(structured.Flights)+len(structured.Invoices) > 0 {
		data, _ := json.Marshal(structured)
		userMsg += "\nStructured: " + string(data)
	}
	userMsg += "\nBody:\n" + body
	return userMsg, ""
}

// Count recipient addresses rather than commas inside quoted display names.
func mailRecipients(to, cc string) (int, error) {
	total := 0
	for _, field := range []string{to, cc} {
		if strings.TrimSpace(field) == "" {
			continue
		}
		addrs, err := netmail.ParseAddressList(field)
		if err != nil {
			return 2, err
		} // malformed envelopes require manual approval
		total += len(addrs)
	}
	return total, nil
}
