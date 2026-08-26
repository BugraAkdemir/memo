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

// ─── Wire message shapes (BidiGenerateContent, confirmed against current
// API docs 2026-08-26) ───────────────────────────────────────────────

type clientMessage struct {
	Setup         *setupMessage         `json:"setup,omitempty"`
	RealtimeInput *realtimeInputMessage `json:"realtimeInput,omitempty"`
	ToolResponse  *toolResponseMessage  `json:"toolResponse,omitempty"`
}

type setupMessage struct {
	Model              string             `json:"model"`
	ResponseModalities []string           `json:"responseModalities"`
	SystemInstruction  *systemInstruction `json:"systemInstruction,omitempty"`
	Tools              []setupTool        `json:"tools,omitempty"`
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
	Audio *audioChunk `json:"audio"`
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

type serverContent struct {
	ModelTurn    *modelTurn `json:"modelTurn,omitempty"`
	TurnComplete bool       `json:"turnComplete,omitempty"`
	Interrupted  bool       `json:"interrupted,omitempty"`
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
	c.conn = conn

	setup := clientMessage{Setup: &setupMessage{
		Model:              c.model,
		ResponseModalities: []string{"AUDIO"},
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

		if msg.ToolCall != nil {
			for _, fc := range msg.ToolCall.FunctionCalls {
				go c.runToolCall(fc)
			}
			continue
		}

		if msg.ServerContent == nil || msg.ServerContent.ModelTurn == nil {
			continue
		}
		for _, part := range msg.ServerContent.ModelTurn.Parts {
			if part.InlineData == nil {
				continue
			}
			audio, err := base64.StdEncoding.DecodeString(part.InlineData.Data)
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
