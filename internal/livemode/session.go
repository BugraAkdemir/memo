package livemode

import "context"

// SessionEventType discriminates SessionEvent's payload, the same
// convention api.StreamChunk's FinishReason already uses for chat
// streaming (internal/api/types.go) — a small, closed set of event kinds
// multiplexed over one channel rather than separate typed channels.
type SessionEventType string

const (
	// EventAudioOut carries a raw PCM chunk the engine wants played back to
	// the user.
	EventAudioOut SessionEventType = "audio_out"
	// EventTranscript carries a partial or final transcript of what was
	// said — see SessionEvent.Role for who (both providers surface this
	// even while using native audio — see
	// docs/plans/PLAN_live_mode_v2.md §5.2).
	EventTranscript SessionEventType = "transcript"
	// EventFunctionCall carries a tool-call request from the engine — the
	// delegate_to_main_model tool (or, in "standalone" WorkMode, any real
	// agent tool). Wired up in a later phase; this phase's EchoSession
	// never emits it.
	EventFunctionCall SessionEventType = "function_call"
	EventError        SessionEventType = "error"
	EventClosed       SessionEventType = "closed"
)

// RoleUser/RoleModel identify who a SessionEvent.Transcript belongs to —
// the user's own speech vs. the model's spoken reply, transcribed by the
// provider itself. Added after real-world testing surfaced that the Live
// Mode UI needs to display a live conversation as a normal chat (see
// docs/plans/PLAN_live_mode_v2.md's follow-up plan), which requires
// telling the two apart; before this, EventTranscript only ever carried
// the user's side.
const (
	RoleUser  = "user"
	RoleModel = "model"
)

// SessionEvent is the typed union of everything a Session can emit.
type SessionEvent struct {
	Type SessionEventType

	Audio []byte // EventAudioOut

	Transcript string // EventTranscript
	Role       string // EventTranscript only — RoleUser or RoleModel

	FunctionCallID   string // EventFunctionCall
	FunctionCallName string
	FunctionCallArgs []byte // raw JSON

	Err error // EventError
}

// Session is the common contract a realtime engine session exposes to
// internal/webserver's WS bridge handler (handleLiveModeSession) —
// implemented by google.Session/openai_realtime.Session in later phases,
// and by EchoSession in this phase to prove the Flutter<->backend
// transport end-to-end before either real provider exists. See
// docs/plans/PLAN_live_mode_v2.md's Phase 6.
type Session interface {
	// Start begins the session. For a real engine this opens the outbound
	// WebSocket to the provider and sends its initial setup message; for
	// EchoSession it's a no-op.
	Start(ctx context.Context) error
	// SendAudio forwards one raw PCM chunk from the user's microphone into
	// the session.
	SendAudio(pcm []byte) error
	// InjectContext sends a short, out-of-turn text aside into the open
	// session — both providers document this as a standard mechanism
	// (Google's realtimeInput.text, OpenAI's conversation.item.create with
	// a system-role message item), confirmed against current API docs,
	// 2026-08-26. Used for mid-session memory refresh (see
	// docs/plans/PLAN_live_mode_v2.md §5.2) and, in a later phase once
	// live-verified, delegated-task progress narration. EchoSession's
	// implementation is a no-op.
	InjectContext(text string) error
	// Events is the channel of everything the session produces — closed
	// once the session is done (after an EventClosed event, or immediately
	// on an unrecoverable error).
	Events() <-chan SessionEvent
	// Close ends the session and releases any underlying connection.
	Close() error
}
