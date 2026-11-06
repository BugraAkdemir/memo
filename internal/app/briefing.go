package app

import (
	"context"
	"encoding/json"
	"time"

	"memo/internal/autonomy"
	"memo/internal/logx"
)

// runBriefings builds whatever briefing is due at now: the morning card once
// the owner's local clock reaches the chosen time, and a note for each meeting
// that starts within the lead time. Both are assembled from local data and each
// is announced once. A model is called only when the owner chose a briefing
// model, and only to write the morning card's summary and custom items.
func (a *App) runBriefings(ctx context.Context, now time.Time) (int, error) {
	a.twinMu.RLock()
	store := a.twinStore
	a.twinMu.RUnlock()
	if store == nil {
		return 0, nil
	}
	prefs, err := store.Preferences(ctx)
	if err != nil || (!prefs.Briefing.Morning && !prefs.Briefing.PreMeeting) {
		return 0, err
	}
	b := &autonomy.Briefer{Store: store, Now: func() time.Time { return now }}
	if a.calendarStore != nil {
		b.Calendar = a.calendarStore
	}
	// Reading the working tree is the same permission as the Git tools.
	if prefs.AllowProjectInspection {
		b.Dev = func(ctx context.Context) []string { return autonomy.DevWatch(ctx, a.twinProjects()) }
	}
	// The owner's briefing model, when one is chosen. A failure only costs the
	// prose: the plain card is still saved.
	if m := prefs.Briefing.Model; m.Provider != "" && m.Model != "" {
		compose := a.briefingComposer(m)
		b.Compose = func(ctx context.Context, in autonomy.ComposeInput) (autonomy.ComposeOutput, error) {
			out, err := compose(ctx, in)
			if err != nil {
				logx.Printf("briefing: model failed, using the plain card: %v", err)
			}
			return out, err
		}
	}
	count := 0
	announce := func(key, kind string) {
		// The ambient bus carries an id and a kind, never a title or a person.
		raw, _ := json.Marshal(map[string]string{"key": key, "kind": kind})
		a.emitEvent("autonomy:briefing", string(raw))
		count++
	}
	morning, err := b.Morning(ctx)
	if err != nil {
		return count, err
	}
	if morning != nil {
		announce(morning.Key, morning.Kind)
	}
	notes, err := b.MeetingNotes(ctx)
	for _, n := range notes {
		announce(n.Key, n.Kind)
	}
	return count, err
}
