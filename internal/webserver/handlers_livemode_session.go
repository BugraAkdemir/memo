package webserver

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/coder/websocket"
	"memo/internal/livemode"
)

// liveModeSessionControlFrame is the JSON shape a text WS message carries —
// every SessionEvent except EventAudioOut (which rides a binary message
// instead, so the raw PCM never round-trips through base64/JSON). Mirrors
// the api.StreamChunk FinishReason-as-discriminator convention this
// codebase already uses for chat SSE (see AGENTS.md's Streaming gotcha).
type liveModeSessionControlFrame struct {
	Type             string `json:"type"`
	Transcript       string `json:"transcript,omitempty"`
	FunctionCallID   string `json:"function_call_id,omitempty"`
	FunctionCallName string `json:"function_call_name,omitempty"`
	FunctionCallArgs string `json:"function_call_args,omitempty"`
	Error            string `json:"error,omitempty"`
}

// handleLiveModeSession bridges the Flutter client's WebSocket to a
// livemode.Session — Phase 6 always uses livemode.EchoSession regardless of
// the configured active engine, to prove the transport (binary PCM
// audio both directions, JSON control frames for everything else) works
// end-to-end before either real provider client exists. Wiring this up to
// the actual active engine (Google Live/OpenAI Realtime) is Phase 7/8. See
// docs/plans/PLAN_live_mode_v2.md.
func (s *Server) handleLiveModeSession(w http.ResponseWriter, r *http.Request) {
	if s.fullBridge == nil {
		http.Error(w, "not available", http.StatusNotImplemented)
		return
	}

	c, err := websocket.Accept(w, r, nil)
	if err != nil {
		return
	}
	defer c.CloseNow()

	ctx := r.Context()
	session := livemode.NewEchoSession()
	if err := session.Start(ctx); err != nil {
		c.Close(websocket.StatusInternalError, err.Error())
		return
	}
	defer session.Close()

	writeDone := make(chan struct{})
	go pumpLiveModeSessionEvents(ctx, c, session, writeDone)

	pumpLiveModeSessionAudio(ctx, c, session)
	session.Close()
	<-writeDone
}

// pumpLiveModeSessionEvents forwards every SessionEvent to the client:
// EventAudioOut as a binary message, everything else as a JSON text frame.
// Returns (closing writeDone) once the session's Events channel closes or
// a write fails.
func pumpLiveModeSessionEvents(ctx context.Context, c *websocket.Conn, session livemode.Session, writeDone chan<- struct{}) {
	defer close(writeDone)
	for {
		select {
		case ev, ok := <-session.Events():
			if !ok {
				return
			}
			if ev.Type == livemode.EventAudioOut {
				if err := c.Write(ctx, websocket.MessageBinary, ev.Audio); err != nil {
					return
				}
				continue
			}
			frame := liveModeSessionControlFrame{Type: string(ev.Type), Transcript: ev.Transcript}
			if ev.Type == livemode.EventFunctionCall {
				frame.FunctionCallID = ev.FunctionCallID
				frame.FunctionCallName = ev.FunctionCallName
				frame.FunctionCallArgs = string(ev.FunctionCallArgs)
			}
			if ev.Err != nil {
				frame.Error = ev.Err.Error()
			}
			payload, err := json.Marshal(frame)
			if err != nil {
				return
			}
			if err := c.Write(ctx, websocket.MessageText, payload); err != nil {
				return
			}
		case <-ctx.Done():
			return
		}
	}
}

// pumpLiveModeSessionAudio reads client->server frames until the
// connection closes: binary messages are forwarded to the session as
// microphone audio, text messages are ignored (no client->server control
// frames exist yet in this phase).
func pumpLiveModeSessionAudio(ctx context.Context, c *websocket.Conn, session livemode.Session) {
	for {
		msgType, data, err := c.Read(ctx)
		if err != nil {
			return
		}
		if msgType != websocket.MessageBinary {
			continue
		}
		if err := session.SendAudio(data); err != nil {
			return
		}
	}
}
