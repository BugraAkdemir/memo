package app

import (
	"context"
	"time"

	"memo/internal/logx"
	"memo/internal/twin"
)

// waitingView is what the UI may know about a pending reply: who, which
// thread and when, never what the owner wrote.
type waitingView struct {
	ID      string    `json:"id"`
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
		out = append(out, waitingView{c.ID, c.Channel, c.Address, c.Subject, c.Status, c.CreatedAt, c.DueAt})
	}
	return out
}

// The radar only prepares reminder drafts for owner review, so a coarse tick is
// enough; the first scan waits so startup is not slowed by a model call.
const (
	followUpFirstScan = time.Minute
	followUpInterval  = 5 * time.Minute
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
