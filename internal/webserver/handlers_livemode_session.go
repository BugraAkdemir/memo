package webserver

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/coder/websocket"
	"memo/internal/livemode"
	"memo/internal/logx"
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
// livemode.Session. Which concrete Session (a real google.Client/
// openai_realtime.Client, wired up with this WorkMode's tools/tool-call
// handling/session-start system prompt, or the livemode.EchoSession
// fallback) is entirely FullBridge.NewLiveModeSession's decision (App,
// Phase 10) — this handler only owns the WS transport, unchanged since
// Phase 6. See docs/plans/PLAN_live_mode_v2.md.
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
	session := s.fullBridge.NewLiveModeSession(ctx)
	if err := session.Start(ctx); err != nil {
		logx.Printf("livemode session: Start failed: %v", err)
		c.Close(websocket.StatusInternalError, err.Error())
		return
	}
	logx.Printf("livemode session: started")
	defer session.Close()

	writeDone := make(chan struct{})
	go pumpLiveModeSessionEvents(ctx, c, session, writeDone)

	framesIn := pumpLiveModeSessionAudio(ctx, c, session)
	session.Close()
	<-writeDone
	logx.Printf("livemode session: closed (%d audio frames received from client)", framesIn)
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
			if ev.Type == livemode.EventError {
				logx.Printf("livemode session: EventError: %v", ev.Err)
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
// frames exist yet in this phase). Returns the number of audio frames
// successfully forwarded — logged by the caller so a session that
// connects but never actually receives mic audio (a silent capture-side
// failure, as opposed to a session/protocol failure) is distinguishable
// from the terminal output alone.
func pumpLiveModeSessionAudio(ctx context.Context, c *websocket.Conn, session livemode.Session) int {
	frames := 0
	for {
		msgType, data, err := c.Read(ctx)
		if err != nil {
			if frames == 0 {
				logx.Printf("livemode session: read loop ended before any audio frame arrived: %v", err)
			}
			return frames
		}
		if msgType != websocket.MessageBinary {
			continue
		}
		if frames == 0 {
			// Confirms real-sized audio is actually flowing (not silently
			// empty/near-empty frames) without logging every single frame
			// — see readLoop's own per-message diagnostic (google/client.go)
			// for the matching "is Google saying anything back" half of
			// this question.
			logx.Printf("livemode session: first audio frame from client: %d bytes", len(data))
		}
		if err := session.SendAudio(data); err != nil {
			logx.Printf("livemode session: SendAudio failed after %d frame(s): %v", frames, err)
			return frames
		}
		frames++
	}
}
