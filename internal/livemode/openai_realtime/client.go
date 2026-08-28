package openai_realtime

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/coder/websocket"
	"memo/internal/livemode"
)

// SessionBaseURL is a var (not const) so tests can point it at an
// httptest-backed WS server instead of the real OpenAI Realtime API.
var SessionBaseURL = "wss://api.openai.com/v1/realtime"

// inputSampleRateHz is OpenAI Realtime's documented PCM sample rate
// (confirmed against current API docs, 2026-08-26: audio/pcm at 24000Hz)
// — unlike Google Live, OpenAI uses the same rate for input and output,
// so there is no separate output constant.
const inputSampleRateHz = 24000

// wsReadLimitBytes overrides coder/websocket's 32768-byte default read
// limit — the same fix google.Client needed after a real live connection
// showed the default silently closes the socket (StatusMessageTooBig) on
// any base64-encoded audio chunk over ~24KB pre-encoding; OpenAI's
// response.output_audio.delta faces the identical size math (24kHz 16-bit
// PCM, base64-inflated) even though this package hasn't been live-tested
// yet — applying the fix here too rather than waiting to rediscover the
// same bug independently. See google/client.go's wsReadLimitBytes for the
// full sizing rationale.
const wsReadLimitBytes = 10 * 1024 * 1024

// ─── Wire message shapes (confirmed against current API docs 2026-08-26)
// ───────────────────────────────────────────────────────────────────

type sessionUpdateEvent struct {
	Type    string        `json:"type"` // "session.update"
	Session sessionConfig `json:"session"`
}

type sessionConfig struct {
	Type             string         `json:"type"` // "realtime" — the session object's own type marker, distinct from the outer event's "type"
	Model            string         `json:"model"`
	OutputModalities []string       `json:"output_modalities,omitempty"`
	Instructions     string         `json:"instructions,omitempty"`
	Audio            *sessionAudio  `json:"audio,omitempty"`
	Tools            []realtimeTool `json:"tools,omitempty"`
	ToolChoice       string         `json:"tool_choice,omitempty"`
}

type realtimeTool struct {
	Type        string          `json:"type"` // "function"
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Parameters  json.RawMessage `json:"parameters,omitempty"`
}

type sessionAudio struct {
	Input  *sessionAudioInput  `json:"input,omitempty"`
	Output *sessionAudioOutput `json:"output,omitempty"`
}

type sessionAudioInput struct {
	Format *audioFormat `json:"format,omitempty"`
	// Transcription: confirmed shape (current API docs, 2026-08-26) —
	// {"model":"whisper-1"} enables ASR transcripts of the user's speech via
	// conversation.item.input_audio_transcription.completed events, so
	// EventTranscript is available for Live Mode's voice-based permission
	// prompting (Phase 12) and future mid-session memory refresh. Always
	// enabled by this package, mirroring google.Client's InputAudioTranscription.
	Transcription *audioTranscriptionConfig `json:"transcription,omitempty"`
}

type audioTranscriptionConfig struct {
	Model string `json:"model"`
}

type sessionAudioOutput struct {
	Format *audioFormat `json:"format,omitempty"`
	// Voice selects a named voice (confirmed against current API docs,
	// 2026-08-27: alloy, ash, ballad, coral, echo, sage, shimmer, verse,
	// marin, cedar — marin/cedar recommended by OpenAI for best quality;
	// OpenAI documents that the voice cannot change once the model has
	// already emitted audio in a session). Empty means the provider's own
	// default.
	Voice string `json:"voice,omitempty"`
}

type audioFormat struct {
	Type string `json:"type"` // "audio/pcm"
	Rate int    `json:"rate,omitempty"`
}

type inputAudioBufferAppendEvent struct {
	Type  string `json:"type"`  // "input_audio_buffer.append"
	Audio string `json:"audio"` // base64
}

// conversationItemCreateEvent + functionCallOutputItem send a tool call's
// result back — confirmed shape (current API docs, 2026-08-26):
// {"type":"conversation.item.create","item":{"type":"function_call_output",
// "call_id":"...","output":"..."}}. responseCreateEvent
// ({"type":"response.create"}) must follow immediately so the model
// actually continues speaking with the result, per the same docs.
type conversationItemCreateEvent struct {
	Type string                 `json:"type"` // "conversation.item.create"
	Item functionCallOutputItem `json:"item"`
}

type functionCallOutputItem struct {
	Type   string `json:"type"` // "function_call_output"
	CallID string `json:"call_id"`
	Output string `json:"output"`
}

type responseCreateEvent struct {
	Type string `json:"type"` // "response.create"
}

// conversationItemCreateMessageEvent + messageItem inject an out-of-turn
// text aside (InjectContext) — a second, distinct conversation.item.create
// shape from conversationItemCreateEvent above (that one's Item is always a
// function_call_output; this one's is always a system-role message). Kept
// as separate types rather than a shared polymorphic Item field so each
// stays a plain, directly-marshalable struct.
type conversationItemCreateMessageEvent struct {
	Type string      `json:"type"` // "conversation.item.create"
	Item messageItem `json:"item"`
}

type messageItem struct {
	Type    string               `json:"type"` // "message"
	Role    string               `json:"role"` // "system"
	Content []messageContentPart `json:"content"`
}

type messageContentPart struct {
	Type string `json:"type"` // "input_text"
	Text string `json:"text"`
}

// serverEvent covers only the fields this phase's client reads —
// response.output_audio.delta's "delta" field, and
// response.function_call_arguments.done's call_id/name/arguments. Every
// other event type (session.created, session.updated, input_audio_buffer.
// speech_started/stopped, etc.) is read and silently ignored by Type not
// matching, the same defensive "unknown/irrelevant message is not fatal"
// stance google.Client's readLoop takes.
type serverEvent struct {
	Type       string `json:"type"`
	Delta      string `json:"delta,omitempty"`
	CallID     string `json:"call_id,omitempty"`
	Name       string `json:"name,omitempty"`
	Arguments  string `json:"arguments,omitempty"`
	Transcript string `json:"transcript,omitempty"`
}

const (
	serverEventAudioDelta                 = "response.output_audio.delta"
	serverEventFunctionCallArgsDone       = "response.function_call_arguments.done"
	serverEventInputTranscriptionComplete = "conversation.item.input_audio_transcription.completed"
	// serverEventOutputTranscriptDone carries the model's own spoken
	// reply, transcribed, as one complete string (as opposed to
	// response.output_audio_transcript.delta's incremental chunks, which
	// this package doesn't need — the Live Mode UI displays one chat
	// bubble per utterance, not a live-typing effect). Confirmed against
	// current API docs, 2026-08-26 (GA interface event naming, matching
	// this package's own output_modalities/OutputAudio naming already in
	// use — the field itself is named "transcript", the same name
	// conversation.item.input_audio_transcription.completed already uses).
	serverEventOutputTranscriptDone = "response.output_audio_transcript.done"
)

// ─── Client ───────────────────────────────────────────────────────────

// Client is an OpenAI Realtime session — implements livemode.Session.
// Mirrors internal/livemode/google.Client's shape exactly (setup/audio +
// tool declarations/function-call handling, see runToolCall), against
// OpenAI's own message names.
type Client struct {
	apiKey         string
	model          string // realtime-family model ID a discovery call (ListRealtimeModels) returned — never guessed here
	instructions   string
	voice          string // e.g. "marin" — empty means the provider's own default
	tools          []livemode.ToolSpec
	handleToolCall livemode.ToolCallHandler

	conn   *websocket.Conn
	ctx    context.Context
	cancel context.CancelFunc

	events    chan livemode.SessionEvent
	closeOnce sync.Once
}

var _ livemode.Session = (*Client)(nil)

// NewClient constructs a Client. model must be a realtime-capable model ID
// (see ListRealtimeModels) — this package never guesses one.
// tools/handleToolCall may both be nil (a session with no delegation
// capability at all — not a normal configuration, but not an error either).
// voice is optional (variadic so every existing call site — mostly tests —
// keeps compiling unchanged): pass a voice name (e.g. "marin", "alloy" —
// confirmed against current API docs, 2026-08-27) to override the
// provider's own default voice.
func NewClient(apiKey, model, instructions string, tools []livemode.ToolSpec, handleToolCall livemode.ToolCallHandler, voice ...string) *Client {
	v := ""
	if len(voice) > 0 {
		v = voice[0]
	}
	return &Client{
		apiKey:         apiKey,
		model:          model,
		instructions:   instructions,
		voice:          v,
		tools:          tools,
		handleToolCall: handleToolCall,
		events:         make(chan livemode.SessionEvent, 16),
	}
}

func (c *Client) Start(ctx context.Context) error {
	sessionCtx, cancel := context.WithCancel(ctx)
	c.ctx = sessionCtx
	c.cancel = cancel

	dialURL := SessionBaseURL + "?model=" + c.model
	conn, _, err := websocket.Dial(sessionCtx, dialURL, &websocket.DialOptions{
		HTTPHeader: map[string][]string{"Authorization": {"Bearer " + c.apiKey}},
	})
	if err != nil {
		cancel()
		return fmt.Errorf("livemode openai: dial: %w", err)
	}
	conn.SetReadLimit(wsReadLimitBytes)
	c.conn = conn

	update := sessionUpdateEvent{
		Type: "session.update",
		Session: sessionConfig{
			Type:             "realtime",
			Model:            c.model,
			OutputModalities: []string{"audio"},
			Instructions:     c.instructions,
			Audio: &sessionAudio{
				Input:  &sessionAudioInput{Format: &audioFormat{Type: "audio/pcm", Rate: inputSampleRateHz}, Transcription: &audioTranscriptionConfig{Model: "whisper-1"}},
				Output: &sessionAudioOutput{Format: &audioFormat{Type: "audio/pcm"}, Voice: c.voice},
			},
		},
	}
	if len(c.tools) > 0 {
		tools := make([]realtimeTool, 0, len(c.tools))
		for _, t := range c.tools {
			tools = append(tools, realtimeTool{Type: "function", Name: t.Name, Description: t.Description, Parameters: t.Parameters})
		}
		update.Session.Tools = tools
		update.Session.ToolChoice = "auto"
	}
	if err := c.writeJSON(update); err != nil {
		cancel()
		conn.Close(websocket.StatusInternalError, "session.update failed")
		return fmt.Errorf("livemode openai: send session.update: %w", err)
	}

	go c.readLoop()
	return nil
}

// SendAudio forwards one raw PCM chunk (16-bit LE, 24kHz — the caller's
// responsibility, this package does no resampling) as an
// input_audio_buffer.append event.
func (c *Client) SendAudio(pcm []byte) error {
	return c.writeJSON(inputAudioBufferAppendEvent{
		Type:  "input_audio_buffer.append",
		Audio: base64.StdEncoding.EncodeToString(pcm),
	})
}

// InjectContext sends text as a conversation.item.create system-role
// message item — confirmed (current API docs, 2026-08-26) as the standard
// way to inject context dynamically into an open Realtime session without
// a full instructions rewrite. See Phase 11, docs/plans/PLAN_live_mode_v2.md
// §5.2.
func (c *Client) InjectContext(text string) error {
	return c.writeJSON(conversationItemCreateMessageEvent{
		Type: "conversation.item.create",
		Item: messageItem{
			Type: "message",
			Role: "system",
			Content: []messageContentPart{
				{Type: "input_text", Text: text},
			},
		},
	})
}

func (c *Client) Events() <-chan livemode.SessionEvent { return c.events }

func (c *Client) Close() error {
	var err error
	c.closeOnce.Do(func() {
		if c.cancel != nil {
			c.cancel()
		}
		if c.conn != nil {
			err = c.conn.Close(websocket.StatusNormalClosure, "")
		}
	})
	return err
}

func (c *Client) writeJSON(v any) error {
	payload, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return c.conn.Write(c.ctx, websocket.MessageText, payload)
}

// readLoop decodes server events until the connection closes, emitting
// EventAudioOut for each response.output_audio.delta and EventTranscript
// for both directions (user speech, model's own spoken reply). Every other
// event type is skipped — see serverEvent's doc comment.
func (c *Client) readLoop() {
	defer close(c.events)
	for {
		_, data, err := c.conn.Read(c.ctx)
		if err != nil {
			select {
			case c.events <- livemode.SessionEvent{Type: livemode.EventError, Err: err}:
			case <-c.ctx.Done():
			}
			return
		}

		var ev serverEvent
		if err := json.Unmarshal(data, &ev); err != nil {
			continue
		}

		switch ev.Type {
		case serverEventFunctionCallArgsDone:
			go c.runToolCall(ev)
			continue
		case serverEventInputTranscriptionComplete:
			if ev.Transcript == "" {
				continue
			}
			select {
			case c.events <- livemode.SessionEvent{Type: livemode.EventTranscript, Role: livemode.RoleUser, Transcript: ev.Transcript}:
			case <-c.ctx.Done():
				return
			}
		case serverEventOutputTranscriptDone:
			// No explicit "enable output transcription" session field was
			// found in current docs the way input's audio.input.transcription
			// exists — the model generates the text it's speaking directly,
			// unlike input (a separate ASR pass over raw user audio), so
			// this is expected to arrive automatically once
			// output_modalities includes "audio" (already the case, see
			// Start()). Unverified against a real session yet (only
			// google.Client's equivalent has been live-tested so far) — if
			// this never fires in practice, that's the one thing to
			// revisit here.
			transcript := livemode.SanitizeModelTranscript(ev.Transcript)
			if transcript == "" {
				continue
			}
			select {
			case c.events <- livemode.SessionEvent{Type: livemode.EventTranscript, Role: livemode.RoleModel, Transcript: transcript}:
			case <-c.ctx.Done():
				return
			}
		case serverEventAudioDelta:
			if ev.Delta == "" {
				continue
			}
			audio, err := base64.StdEncoding.DecodeString(ev.Delta)
			if err != nil {
				continue
			}
			select {
			case c.events <- livemode.SessionEvent{Type: livemode.EventAudioOut, Audio: audio}:
			case <-c.ctx.Done():
				return
			}
		}
	}
}

// runToolCall resolves one function call via handleToolCall and sends the
// result back as conversation.item.create + response.create — run in its
// own goroutine (from readLoop) so a slow delegated task never blocks the
// read loop from processing further server messages in the meantime. A
// nil handleToolCall reports back "not supported" rather than silently
// hanging the model waiting for a response that will never come.
func (c *Client) runToolCall(ev serverEvent) {
	var result string
	if c.handleToolCall == nil {
		result = "Error: tool calling is not available in this session"
	} else if r, err := c.handleToolCall(c.ctx, ev.Name, json.RawMessage(ev.Arguments)); err != nil {
		result = "Error: " + err.Error()
	} else {
		result = r
	}

	if err := c.writeJSON(conversationItemCreateEvent{
		Type: "conversation.item.create",
		Item: functionCallOutputItem{Type: "function_call_output", CallID: ev.CallID, Output: result},
	}); err != nil {
		select {
		case c.events <- livemode.SessionEvent{Type: livemode.EventError, Err: err}:
		case <-c.ctx.Done():
		}
		return
	}
	if err := c.writeJSON(responseCreateEvent{Type: "response.create"}); err != nil {
		select {
		case c.events <- livemode.SessionEvent{Type: livemode.EventError, Err: err}:
		case <-c.ctx.Done():
		}
	}
}
