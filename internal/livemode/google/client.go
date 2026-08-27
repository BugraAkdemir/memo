package google

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"sync"

	"github.com/coder/websocket"
	"memo/internal/livemode"
	"memo/internal/logx"
)

// SessionBaseURL is a var (not const) so tests can point it at an
// httptest-backed WS server instead of the real Gemini Live API.
var SessionBaseURL = "wss://generativelanguage.googleapis.com/ws/google.ai.generativelanguage.v1beta.GenerativeService.BidiGenerateContent"

// inputSampleRateHz/outputSampleRateHz match the Live API's documented
// fixed audio contract (confirmed against current docs, 2026-08-26): input
// is always 16-bit PCM little-endian at 16kHz, output always 16-bit PCM
// little-endian at 24kHz — neither is negotiable per-session.
const (
	inputSampleRateHz  = 16000
	outputSampleRateHz = 24000
)

// wsReadLimitBytes overrides coder/websocket's 32768-byte default read
// limit — found via a real live connection: a serverContent audio chunk's
// base64-encoded inlineData routinely exceeds 32KB (24kHz 16-bit PCM ~=
// 48000 bytes/sec pre-base64, ~64000 bytes/sec after base64's ~33%
// inflation — well under a second of audio already blows the default
// limit), and the default silently closes the connection with
// StatusMessageTooBig instead of erroring cleanly. 10MB is a generous
// safety margin for any single message (audio chunk, tool-call args, or
// otherwise) without being unbounded — this is an outbound connection to a
// trusted first-party API, not a server accepting arbitrary client input,
// so the usual "don't trust a huge advertised size" concern doesn't apply
// the same way here.
const wsReadLimitBytes = 10 * 1024 * 1024

// ─── Wire message shapes (BidiGenerateContent, confirmed against current
// API docs 2026-08-26) ───────────────────────────────────────────────

type clientMessage struct {
	Setup         *setupMessage         `json:"setup,omitempty"`
	RealtimeInput *realtimeInputMessage `json:"realtimeInput,omitempty"`
	ToolResponse  *toolResponseMessage  `json:"toolResponse,omitempty"`
}

type setupMessage struct {
	Model string `json:"model"`
	// GenerationConfig.ResponseModalities: NOT a direct field of setup —
	// found the hard way via a real live connection (the server closed
	// with "Invalid JSON payload received. Unknown name \"responseModalities\"
	// at 'setup': Cannot find field."). Confirmed against the official
	// reference (ai.google.dev/api/live, fetched 2026-08-26):
	// responseModalities nests inside generationConfig; model/
	// systemInstruction/tools/inputAudioTranscription/
	// outputAudioTranscription are all direct setup fields (those were
	// already right).
	GenerationConfig  *generationConfig  `json:"generationConfig,omitempty"`
	SystemInstruction *systemInstruction `json:"systemInstruction,omitempty"`
	Tools             []setupTool        `json:"tools,omitempty"`
	// InputAudioTranscription/OutputAudioTranscription: an empty object
	// enables ASR transcripts for that direction (confirmed shape, current
	// API docs, 2026-08-26) — always enabled by this package so
	// EventTranscript is available for Live Mode's voice-based permission
	// prompting (Phase 12) and future mid-session memory refresh.
	InputAudioTranscription  *struct{} `json:"inputAudioTranscription,omitempty"`
	OutputAudioTranscription *struct{} `json:"outputAudioTranscription,omitempty"`
}

type generationConfig struct {
	ResponseModalities []string `json:"responseModalities"`
}

type setupTool struct {
	FunctionDeclarations []functionDeclaration `json:"functionDeclarations"`
}

type functionDeclaration struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Parameters  json.RawMessage `json:"parameters,omitempty"`
}

type systemInstruction struct {
	Parts []messagePart `json:"parts"`
}

type messagePart struct {
	Text string `json:"text,omitempty"`
}

type realtimeInputMessage struct {
	Audio *audioChunk `json:"audio,omitempty"`
	// Text injects an out-of-turn aside into the open session — confirmed
	// shape (current API docs, 2026-08-26): {"realtimeInput":{"text":"..."}}.
	// See InjectContext.
	Text string `json:"text,omitempty"`
}

type audioChunk struct {
	Data     string `json:"data"` // base64
	MimeType string `json:"mimeType"`
}

type serverMessage struct {
	SetupComplete *struct{}      `json:"setupComplete,omitempty"`
	ServerContent *serverContent `json:"serverContent,omitempty"`
	ToolCall      *toolCall      `json:"toolCall,omitempty"`
}

type toolCall struct {
	FunctionCalls []functionCall `json:"functionCalls"`
}

type functionCall struct {
	ID   string          `json:"id"`
	Name string          `json:"name"`
	Args json.RawMessage `json:"args"`
}

type toolResponseMessage struct {
	FunctionResponses []functionResponse `json:"functionResponses"`
}

type functionResponse struct {
	ID       string                 `json:"id"`
	Name     string                 `json:"name"`
	Response map[string]interface{} `json:"response"`
}

// serverContent.InputTranscription/OutputTranscription: nested here rather
// than as top-level serverMessage fields — the current docs handed
// genuinely inconsistent signals on this point (a general BidiGenerateContent
// message-shape overview suggested top-level, but transcription-specific
// examples showed response.server_content.input_transcription nested).
// Went with nesting inside serverContent since that's what the
// transcription-specific sources actually demonstrated; if a live session
// shows otherwise, this is the one line to fix (see runToolCall's sibling
// note on the same kind of ambiguity risk).
type serverContent struct {
	ModelTurn           *modelTurn         `json:"modelTurn,omitempty"`
	TurnComplete        bool               `json:"turnComplete,omitempty"`
	Interrupted         bool               `json:"interrupted,omitempty"`
	InputTranscription  *transcriptionText `json:"inputTranscription,omitempty"`
	OutputTranscription *transcriptionText `json:"outputTranscription,omitempty"`
}

type transcriptionText struct {
	Text string `json:"text"`
}

type modelTurn struct {
	Parts []serverPart `json:"parts"`
}

type serverPart struct {
	InlineData *inlineData `json:"inlineData,omitempty"`
}

type inlineData struct {
	MimeType string `json:"mimeType"`
	Data     string `json:"data"` // base64
}

// ─── Client ───────────────────────────────────────────────────────────

// Client is a Google Live (Gemini Live API) realtime session — implements
// livemode.Session, including tool declarations and function-call handling
// for delegation (see runToolCall).
type Client struct {
	apiKey            string
	model             string // e.g. "models/gemini-3.1-flash-live-preview" — the exact Name a discovery call (google.ListLiveModels) returned, never hardcoded by this package.
	systemInstruction string
	tools             []livemode.ToolSpec
	handleToolCall    livemode.ToolCallHandler

	conn   *websocket.Conn
	ctx    context.Context
	cancel context.CancelFunc

	events    chan livemode.SessionEvent
	closeOnce sync.Once
}

var _ livemode.Session = (*Client)(nil)

// NewClient constructs a Client. model must be a Live-capable model's full
// resource name (see ListLiveModels) — this package never guesses one.
// tools/handleToolCall may both be nil (a session with no delegation
// capability at all — not a normal configuration, but not an error either).
func NewClient(apiKey, model, systemInstruction string, tools []livemode.ToolSpec, handleToolCall livemode.ToolCallHandler) *Client {
	return &Client{
		apiKey:            apiKey,
		model:             model,
		systemInstruction: systemInstruction,
		tools:             tools,
		handleToolCall:    handleToolCall,
		events:            make(chan livemode.SessionEvent, 16),
	}
}

func (c *Client) Start(ctx context.Context) error {
	sessionCtx, cancel := context.WithCancel(ctx)
	c.ctx = sessionCtx
	c.cancel = cancel

	dialURL := SessionBaseURL + "?key=" + url.QueryEscape(c.apiKey)
	conn, _, err := websocket.Dial(sessionCtx, dialURL, nil)
	if err != nil {
		cancel()
		return fmt.Errorf("livemode google: dial: %w", err)
	}
	conn.SetReadLimit(wsReadLimitBytes)
	c.conn = conn

	setup := clientMessage{Setup: &setupMessage{
		Model:                    c.model,
		GenerationConfig:         &generationConfig{ResponseModalities: []string{"AUDIO"}},
		InputAudioTranscription:  &struct{}{},
		OutputAudioTranscription: &struct{}{},
	}}
	if c.systemInstruction != "" {
		setup.Setup.SystemInstruction = &systemInstruction{Parts: []messagePart{{Text: c.systemInstruction}}}
	}
	if len(c.tools) > 0 {
		decls := make([]functionDeclaration, 0, len(c.tools))
		for _, t := range c.tools {
			decls = append(decls, functionDeclaration{Name: t.Name, Description: t.Description, Parameters: t.Parameters})
		}
		setup.Setup.Tools = []setupTool{{FunctionDeclarations: decls}}
	}
	if err := c.writeJSON(setup); err != nil {
		cancel()
		conn.Close(websocket.StatusInternalError, "setup failed")
		return fmt.Errorf("livemode google: send setup: %w", err)
	}

	go c.readLoop()
	return nil
}

// SendAudio forwards one raw PCM chunk (16-bit LE, 16kHz — the caller's
// responsibility, this package does no resampling) as a realtimeInput
// message.
func (c *Client) SendAudio(pcm []byte) error {
	msg := clientMessage{RealtimeInput: &realtimeInputMessage{Audio: &audioChunk{
		Data:     base64.StdEncoding.EncodeToString(pcm),
		MimeType: fmt.Sprintf("audio/pcm;rate=%d", inputSampleRateHz),
	}}}
	return c.writeJSON(msg)
}

// InjectContext sends text as a realtimeInput.text message — confirmed
// (current API docs, 2026-08-26) as the standard way to inject an
// out-of-turn text aside into an open Live session. See Phase 11,
// docs/plans/PLAN_live_mode_v2.md §5.2.
func (c *Client) InjectContext(text string) error {
	return c.writeJSON(clientMessage{RealtimeInput: &realtimeInputMessage{Text: text}})
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

// readLoop decodes server messages until the connection closes, emitting
// EventAudioOut for each inlineData audio part. setupComplete/turnComplete/
// interrupted carry no event in this phase (nothing downstream consumes
// them yet — Phase 9-11 add delegation/progress-narration handling here).
// A malformed message is skipped defensively, never treated as fatal (the
// Live API's own message set may grow fields this package doesn't know
// about yet).
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

		var msg serverMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			continue
		}

		// Diagnostic: confirms whether Google is sending anything at all
		// during a session where nothing actionable ever arrives — added
		// after a real test where the server accepted setup and 139 audio
		// frames but the client (this readLoop) never observed anything
		// worth turning into a SessionEvent, leaving no way to tell "the
		// server is truly silent" apart from "the server is acking but we
		// don't forward acks". Summary only, never the raw inlineData
		// payload (base64 audio would flood the log).
		logx.Printf(
			"livemode google: server message: setupComplete=%v toolCall=%v serverContent=%v turnComplete=%v interrupted=%v hasModelTurn=%v hasInputTranscription=%v hasOutputTranscription=%v",
			msg.SetupComplete != nil, msg.ToolCall != nil, msg.ServerContent != nil,
			msg.ServerContent != nil && msg.ServerContent.TurnComplete,
			msg.ServerContent != nil && msg.ServerContent.Interrupted,
			msg.ServerContent != nil && msg.ServerContent.ModelTurn != nil,
			msg.ServerContent != nil && msg.ServerContent.InputTranscription != nil,
			msg.ServerContent != nil && msg.ServerContent.OutputTranscription != nil,
		)

		if msg.ToolCall != nil {
			for _, fc := range msg.ToolCall.FunctionCalls {
				go c.runToolCall(fc)
			}
			continue
		}

		if msg.ServerContent == nil {
			continue
		}

		// User speech transcript.
		if msg.ServerContent.InputTranscription != nil && msg.ServerContent.InputTranscription.Text != "" {
			select {
			case c.events <- livemode.SessionEvent{Type: livemode.EventTranscript, Role: livemode.RoleUser, Transcript: msg.ServerContent.InputTranscription.Text}:
			case <-c.ctx.Done():
				return
			}
		}
		// The model's own spoken reply, as text — the Live Mode UI displays
		// a live conversation as a normal chat (see
		// docs/plans/PLAN_live_mode_v2.md's follow-up plan), which needs
		// this alongside the user's own transcript above.
		if msg.ServerContent.OutputTranscription != nil && msg.ServerContent.OutputTranscription.Text != "" {
			select {
			case c.events <- livemode.SessionEvent{Type: livemode.EventTranscript, Role: livemode.RoleModel, Transcript: msg.ServerContent.OutputTranscription.Text}:
			case <-c.ctx.Done():
				return
			}
		}

		if msg.ServerContent.ModelTurn == nil {
			continue
		}
		for _, part := range msg.ServerContent.ModelTurn.Parts {
			if part.InlineData == nil {
				// Diagnostic: a modelTurn part that isn't audio at all (e.g.
				// a text-only part) — confirms whether ModelTurn.Parts is
				// arriving with a shape this struct doesn't expect.
				logx.Printf("livemode google: modelTurn part with no inlineData")
				continue
			}
			audio, err := base64.StdEncoding.DecodeString(part.InlineData.Data)
			if err != nil {
				logx.Printf("livemode google: failed to decode inlineData (mimeType=%q, len=%d): %v", part.InlineData.MimeType, len(part.InlineData.Data), err)
				continue
			}
			logx.Printf("livemode google: emitting EventAudioOut (mimeType=%q, %d bytes)", part.InlineData.MimeType, len(audio))
			select {
			case c.events <- livemode.SessionEvent{Type: livemode.EventAudioOut, Audio: audio}:
			case <-c.ctx.Done():
				return
			}
		}
	}
}

// runToolCall resolves one function call via handleToolCall and sends the
// result back as a toolResponse message — run in its own goroutine (from
// readLoop) so a slow delegated task never blocks the read loop from
// processing further server messages (audio, other tool calls) in the
// meantime. A nil handleToolCall (a session with no delegation capability
// configured) reports back "not supported" rather than silently hanging
// the model waiting for a response that will never come.
func (c *Client) runToolCall(fc functionCall) {
	var result string
	if c.handleToolCall == nil {
		result = "Error: tool calling is not available in this session"
	} else if r, err := c.handleToolCall(c.ctx, fc.Name, fc.Args); err != nil {
		result = "Error: " + err.Error()
	} else {
		result = r
	}

	resp := clientMessage{ToolResponse: &toolResponseMessage{
		FunctionResponses: []functionResponse{{ID: fc.ID, Name: fc.Name, Response: map[string]interface{}{"result": result}}},
	}}
	if err := c.writeJSON(resp); err != nil {
		select {
		case c.events <- livemode.SessionEvent{Type: livemode.EventError, Err: err}:
		case <-c.ctx.Done():
		}
	}
}
