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

// ─── Wire message shapes (confirmed against current API docs 2026-08-26)
// ───────────────────────────────────────────────────────────────────

type sessionUpdateEvent struct {
	Type    string        `json:"type"` // "session.update"
	Session sessionConfig `json:"session"`
}

type sessionConfig struct {
	Type             string        `json:"type"` // "realtime" — the session object's own type marker, distinct from the outer event's "type"
	Model            string        `json:"model"`
	OutputModalities []string      `json:"output_modalities,omitempty"`
	Instructions     string        `json:"instructions,omitempty"`
	Audio            *sessionAudio `json:"audio,omitempty"`
}

type sessionAudio struct {
	Input  *sessionAudioInput  `json:"input,omitempty"`
	Output *sessionAudioOutput `json:"output,omitempty"`
}

type sessionAudioInput struct {
	Format *audioFormat `json:"format,omitempty"`
}

type sessionAudioOutput struct {
	Format *audioFormat `json:"format,omitempty"`
}

type audioFormat struct {
	Type string `json:"type"` // "audio/pcm"
	Rate int    `json:"rate,omitempty"`
}

type inputAudioBufferAppendEvent struct {
	Type  string `json:"type"`  // "input_audio_buffer.append"
	Audio string `json:"audio"` // base64
}

// serverEvent covers only the fields this phase's client reads —
// response.output_audio.delta's "delta" field. Every other event type
// (session.created, session.updated, input_audio_buffer.speech_started/
// stopped, etc.) is read and silently ignored by Type not matching, the
// same defensive "unknown/irrelevant message is not fatal" stance
// google.Client's readLoop takes.
type serverEvent struct {
	Type  string `json:"type"`
	Delta string `json:"delta,omitempty"`
}

const serverEventAudioDelta = "response.output_audio.delta"

// ─── Client ───────────────────────────────────────────────────────────

// Client is an OpenAI Realtime session — implements livemode.Session.
// Mirrors internal/livemode/google.Client's shape and phase scope exactly
// (setup + audio in/out only; tool declarations/function-call handling for
// delegation land in Phase 10), against OpenAI's own message names.
type Client struct {
	apiKey       string
	model        string // realtime-family model ID a discovery call (ListRealtimeModels) returned — never guessed here
	instructions string

	conn   *websocket.Conn
	ctx    context.Context
	cancel context.CancelFunc

	events    chan livemode.SessionEvent
	closeOnce sync.Once
}

var _ livemode.Session = (*Client)(nil)

// NewClient constructs a Client. model must be a realtime-capable model ID
// (see ListRealtimeModels) — this package never guesses one.
func NewClient(apiKey, model, instructions string) *Client {
	return &Client{
		apiKey:       apiKey,
		model:        model,
		instructions: instructions,
		events:       make(chan livemode.SessionEvent, 16),
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
	c.conn = conn

	update := sessionUpdateEvent{
		Type: "session.update",
		Session: sessionConfig{
			Type:             "realtime",
			Model:            c.model,
			OutputModalities: []string{"audio"},
			Instructions:     c.instructions,
			Audio: &sessionAudio{
				Input:  &sessionAudioInput{Format: &audioFormat{Type: "audio/pcm", Rate: inputSampleRateHz}},
				Output: &sessionAudioOutput{Format: &audioFormat{Type: "audio/pcm"}},
			},
		},
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
// EventAudioOut for each response.output_audio.delta. Every other event
// type is skipped — see serverEvent's doc comment.
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
		if ev.Type != serverEventAudioDelta || ev.Delta == "" {
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
