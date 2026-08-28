package app

import "memo/internal/livemode"

// livePermissionRoutingSession wraps a real livemode.Session so a spoken
// EventTranscript can also resolve a pending voice_prompt permission
// question (via routeLiveTranscriptToPermissionAnswer), while still
// reaching the Flutter client for normal transcript display exactly as
// before — every event is forwarded to the outward Events() channel
// unchanged, transcript or not. See docs/plans/PLAN_live_mode_v2.md Phase
// 12's "dual consumption of transcript events" note and
// liveModeVoicePermissionCallbacks' doc comment.
type livePermissionRoutingSession struct {
	livemode.Session
	a      *App
	events chan livemode.SessionEvent
}

// wrapLiveModeSessionForPermissionRouting wraps s and starts its event
// pump. Always wraps, even for engines/sessions that never emit
// EventTranscript (e.g. EchoSession) — the pump is a harmless passthrough
// in that case.
func (a *App) wrapLiveModeSessionForPermissionRouting(s livemode.Session) livemode.Session {
	w := &livePermissionRoutingSession{Session: s, a: a, events: make(chan livemode.SessionEvent, 16)}
	go w.pump()
	return w
}

func (w *livePermissionRoutingSession) pump() {
	defer close(w.events)
	for ev := range w.Session.Events() {
		if ev.Type == livemode.EventTranscript && ev.Transcript != "" {
			w.a.routeLiveTranscriptToPermissionAnswer(ev.Transcript)
		}
		w.events <- ev
	}
}

func (w *livePermissionRoutingSession) Events() <-chan livemode.SessionEvent { return w.events }
