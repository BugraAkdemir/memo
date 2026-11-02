package app

import (
	"context"
	"encoding/json"
	"time"

	"memo/internal/logx"
	"memo/internal/twin"
)

// waitingView is what the UI may know about a pending reply: who, which
// thread and when, never what the owner wrote.
type waitingView struct {
	ID      string    `json:"id"`
	Kind    string    `json:"kind"`
	Summary string    `json:"summary,omitempty"` // only the owner's own one-line promise
	Channel string    `json:"channel"`
	Address string    `json:"address"`
	Subject string    `json:"subject"`
	Status  string    `json:"status"`
	SentAt  time.Time `json:"sent_at"`
	DueAt   time.Time `json:"due_at"`
}

func waitingViews(list []twin.Commitment) []waitingView {
	out := make([]waitingView, 0, len(list))
	for _, c := range list {
		v := waitingView{ID: c.ID, Kind: c.Kind, Channel: c.Channel, Address: c.Address, Subject: c.Subject, Status: c.Status, SentAt: c.CreatedAt, DueAt: c.DueAt}
		if c.Kind == "my_promise" {
			v.Summary = c.Body
		}
		out = append(out, v)
	}
	return out
}

// The loop only prepares reminder drafts and local briefings. A one-minute tick
// keeps a short pre-meeting window from being missed; the first scan waits so
// startup is not slowed by a model call.
const (
	followUpFirstScan = time.Minute
	followUpInterval  = time.Minute
)

// runFollowUpScan prepares a review-only reminder for every commitment that is
// due at now. Without a chosen reply model nothing is consumed, so the
// reminders are prepared once the owner picks one.
func (a *App) runFollowUpScan(ctx context.Context, now time.Time) (int, error) {
	a.twinMu.RLock()
	defer a.twinMu.RUnlock()
	store := a.twinStore
	if store == nil {
		return 0, nil
	}
	selection, err := store.ReplyModel(ctx)
	if err != nil {
		return 0, err
	}
	engine := a.autonomyEngine(store, selection)
	engine.Now = func() time.Time { return now }
	if selection.Provider == "" || selection.Model == "" {
		engine.Generate = nil
	}
	return engine.PrepareFollowUps(ctx)
}

// extractTwinPromises reads one message the owner sent for promises. Without a
// chosen reply model it does nothing, so the message can be read later.
func (a *App) extractTwinPromises(ctx context.Context, m twin.Message) (int, error) {
	a.twinMu.RLock()
	defer a.twinMu.RUnlock()
	store := a.twinStore
	if store == nil {
		return 0, nil
	}
	selection, err := store.ReplyModel(ctx)
	if err != nil {
		return 0, err
	}
	engine := a.autonomyEngine(store, selection)
	if selection.Provider == "" || selection.Model == "" {
		engine.Generate = nil
	}
	return engine.ExtractPromises(ctx, m)
}

// runPromiseScan tells the owner about promises whose date has arrived. A
// promise is announced once; the owner closes it with "done".
func (a *App) runPromiseScan(ctx context.Context, now time.Time) (int, error) {
	a.twinMu.RLock()
	store := a.twinStore
	a.twinMu.RUnlock()
	if store == nil {
		return 0, nil
	}
	prefs, err := store.Preferences(ctx)
	if err != nil || !prefs.Radar.Enabled || !prefs.Radar.TrackPromises {
		return 0, err
	}
	due, err := store.DuePromises(ctx, now, 50)
	if err != nil {
		return 0, err
	}
	announced := 0
	for _, c := range due {
		won, err := store.SetCommitmentStatus(ctx, c.ID, "open", "reminded")
		if err != nil {
			return announced, err
		}
		if !won {
			continue
		}
		// The ambient event bus never carries the promise text or the contact.
		raw, _ := json.Marshal(map[string]string{"id": c.ID})
		a.emitEvent("autonomy:promise_due", string(raw))
		announced++
	}
	return announced, nil
}

func (a *App) startFollowUpRadar() {
	a.twinMu.Lock()
	if a.twinStore == nil || a.radarCancel != nil || a.lifecycleCtx == nil {
		a.twinMu.Unlock()
		return
	}
	ctx, cancel := context.WithCancel(a.lifecycleCtx)
	done := make(chan struct{})
	a.radarCancel, a.radarDone = cancel, done
	a.twinMu.Unlock()
	goRecover("followUpRadar", func() {
		defer close(done)
		timer := time.NewTimer(followUpFirstScan)
		defer timer.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-timer.C:
				if n, err := a.runBriefings(ctx, time.Now()); err != nil && ctx.Err() == nil {
					logx.Printf("follow-up radar: briefing failed: %v", err)
				} else if n > 0 {
					logx.Printf("follow-up radar: built %d briefing(s)", n)
				}
				if n, err := a.runPromiseScan(ctx, time.Now()); err != nil && ctx.Err() == nil {
					logx.Printf("follow-up radar: promise scan failed: %v", err)
				} else if n > 0 {
					logx.Printf("follow-up radar: %d promise(s) are due", n)
				}
				if n, err := a.runFollowUpScan(ctx, time.Now()); err != nil && ctx.Err() == nil {
					logx.Printf("follow-up radar: scan failed: %v", err)
				} else if n > 0 {
					logx.Printf("follow-up radar: prepared %d reminder draft(s) for review", n)
				}
				timer.Reset(followUpInterval)
			}
		}
	})
}

func (a *App) stopFollowUpRadar() {
	a.twinMu.Lock()
	cancel, done := a.radarCancel, a.radarDone
	a.twinMu.Unlock()
	if cancel != nil {
		cancel()
		<-done
	}
	a.twinMu.Lock()
	a.radarCancel, a.radarDone = nil, nil
	a.twinMu.Unlock()
}
