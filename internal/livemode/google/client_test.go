package google

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"memo/internal/livemode"
)

// fakeLiveServer accepts one WS connection acting like the real Gemini
// Live endpoint: expects a setup message first, records it, then lets the
// test drive further server->client sends via the returned channels.
type fakeLiveServer struct {
	srv        *httptest.Server
	gotSetup   chan setupMessage
	gotAudioIn chan audioChunk
}

func newFakeLiveServer(t *testing.T) *fakeLiveServer {
	t.Helper()
	f := &fakeLiveServer{
		gotSetup:   make(chan setupMessage, 1),
		gotAudioIn: make(chan audioChunk, 8),
	}
	f.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer c.CloseNow()
		ctx := r.Context()

		_, data, err := c.Read(ctx)
		if err != nil {
			return
		}
		var first clientMessage
		if err := json.Unmarshal(data, &first); err != nil || first.Setup == nil {
			t.Errorf("expected the first client message to be a setup message, got %q", data)
			return
		}
		f.gotSetup <- *first.Setup

		for {
			_, data, err := c.Read(ctx)
			if err != nil {
				return
			}
			var msg clientMessage
			if err := json.Unmarshal(data, &msg); err != nil {
				continue
			}
			if msg.RealtimeInput != nil && msg.RealtimeInput.Audio != nil {
				select {
				case f.gotAudioIn <- *msg.RealtimeInput.Audio:
				default:
				}
			}
		}
	}))
	t.Cleanup(f.srv.Close)
	return f
}

func (f *fakeLiveServer) wsURL() string {
	return "ws" + strings.TrimPrefix(f.srv.URL, "http")
}

func TestClient_SendsSetupMessageOnStart(t *testing.T) {
	f := newFakeLiveServer(t)
	original := SessionBaseURL
	SessionBaseURL = f.wsURL()
	defer func() { SessionBaseURL = original }()

	c := NewClient("g-key", "models/gemini-3.1-flash-live-preview", "You are Memo's live voice.", nil, nil)
	if err := c.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer c.Close()

	select {
	case setup := <-f.gotSetup:
		if setup.Model != "models/gemini-3.1-flash-live-preview" {
			t.Errorf("unexpected model in setup: %q", setup.Model)
		}
		if setup.GenerationConfig == nil || len(setup.GenerationConfig.ResponseModalities) != 1 || setup.GenerationConfig.ResponseModalities[0] != "AUDIO" {
			t.Errorf("expected generationConfig.responseModalities=[AUDIO], got %+v", setup.GenerationConfig)
		}
		if setup.SystemInstruction == nil || setup.SystemInstruction.Parts[0].Text != "You are Memo's live voice." {
			t.Errorf("expected systemInstruction to carry the given text, got %+v", setup.SystemInstruction)
		}
		if setup.InputAudioTranscription == nil || setup.OutputAudioTranscription == nil {
			t.Error("expected input/output audio transcription to always be enabled in setup")
		}
		if setup.RealtimeInputConfig == nil ||
			setup.RealtimeInputConfig.AutomaticActivityDetection == nil ||
			setup.RealtimeInputConfig.AutomaticActivityDetection.StartOfSpeechSensitivity != startSensitivityLow {
			t.Errorf("expected realtimeInputConfig.automaticActivityDetection.startOfSpeechSensitivity=%q to lower false-positive barge-in triggers, got %+v", startSensitivityLow, setup.RealtimeInputConfig)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for setup message on the server side")
	}
}

func TestClient_SendAudioForwardsBase64EncodedPCM(t *testing.T) {
	f := newFakeLiveServer(t)
	original := SessionBaseURL
	SessionBaseURL = f.wsURL()
	defer func() { SessionBaseURL = original }()

	c := NewClient("g-key", "models/gemini-3.1-flash-live-preview", "", nil, nil)
	if err := c.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer c.Close()
	<-f.gotSetup // wait past the setup handshake before sending audio

	if err := c.SendAudio([]byte("raw-pcm-bytes")); err != nil {
		t.Fatalf("SendAudio: %v", err)
	}

	select {
	case chunk := <-f.gotAudioIn:
		decoded, err := base64.StdEncoding.DecodeString(chunk.Data)
		if err != nil {
			t.Fatalf("decode: %v", err)
		}
		if string(decoded) != "raw-pcm-bytes" {
			t.Errorf("expected round-tripped PCM bytes, got %q", decoded)
		}
		if chunk.MimeType != "audio/pcm;rate=16000" {
			t.Errorf("expected audio/pcm;rate=16000, got %q", chunk.MimeType)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for audio chunk on the server side")
	}
}

func TestClient_EmitsAudioOutFromServerContent(t *testing.T) {
	f := newFakeLiveServer(t)
	original := SessionBaseURL
	SessionBaseURL = f.wsURL()
	defer func() { SessionBaseURL = original }()

	// Override the fake server to also push a serverContent audio frame
	// right after reading setup — simplest way is a second, dedicated
	// server for this one test rather than complicating fakeLiveServer's
	// shared helper with a send-back mode every other test doesn't need.
	audioB64 := base64.StdEncoding.EncodeToString([]byte("reply-pcm-bytes"))
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer c.CloseNow()
		ctx := r.Context()
		if _, _, err := c.Read(ctx); err != nil { // consume setup
			return
		}
		serverMsg := serverMessage{ServerContent: &serverContent{
			ModelTurn: &modelTurn{Parts: []serverPart{{InlineData: &inlineData{
				MimeType: "audio/pcm;rate=24000",
				Data:     audioB64,
			}}}},
		}}
		payload, _ := json.Marshal(serverMsg)
		c.Write(ctx, websocket.MessageText, payload)
		<-ctx.Done()
	}))
	defer srv.Close()
	SessionBaseURL = "ws" + strings.TrimPrefix(srv.URL, "http")

	c := NewClient("g-key", "models/gemini-3.1-flash-live-preview", "", nil, nil)
	if err := c.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer c.Close()

	select {
	case ev := <-c.Events():
		if ev.Type != "audio_out" {
			t.Fatalf("expected an audio_out event, got %s (err=%v)", ev.Type, ev.Err)
		}
		if string(ev.Audio) != "reply-pcm-bytes" {
			t.Errorf("expected decoded reply audio, got %q", ev.Audio)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for the audio_out event")
	}
}

// TestClient_EmitsTranscriptFromInputTranscription confirms readLoop parses
// serverContent.inputTranscription (the user-speech transcript — see
// serverContent's doc comment on the nesting-location ambiguity this
// package took a stance on) into an EventTranscript, independent of
// whether that same server message also carries a modelTurn audio part.
func TestClient_EmitsTranscriptFromInputTranscription(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer c.CloseNow()
		ctx := r.Context()
		if _, _, err := c.Read(ctx); err != nil { // consume setup
			return
		}
		serverMsg := serverMessage{ServerContent: &serverContent{
			InputTranscription: &transcriptionText{Text: "kapıyı kilitle"},
		}}
		payload, _ := json.Marshal(serverMsg)
		c.Write(ctx, websocket.MessageText, payload)
		<-ctx.Done()
	}))
	defer srv.Close()
	original := SessionBaseURL
	SessionBaseURL = "ws" + strings.TrimPrefix(srv.URL, "http")
	defer func() { SessionBaseURL = original }()

	c := NewClient("g-key", "models/gemini-3.1-flash-live-preview", "", nil, nil)
	if err := c.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer c.Close()

	select {
	case ev := <-c.Events():
		if ev.Type != livemode.EventTranscript {
			t.Fatalf("expected a transcript event, got %s (err=%v)", ev.Type, ev.Err)
		}
		if ev.Transcript != "kapıyı kilitle" {
			t.Errorf("expected the transcript text to round-trip, got %q", ev.Transcript)
		}
		if ev.Role != livemode.RoleUser {
			t.Errorf("expected Role=user for an input transcript, got %q", ev.Role)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for the transcript event")
	}
}

// TestClient_EmitsTranscriptFromOutputTranscription confirms readLoop also
// surfaces the model's own spoken reply (as text) as an EventTranscript
// with Role=model — added for the Live Mode UI's "show the live
// conversation as a normal chat" follow-up (see
// docs/plans/PLAN_live_mode_v2.md), distinct from the user's own
// Role=user transcript above.
func TestClient_EmitsTranscriptFromOutputTranscription(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer c.CloseNow()
		ctx := r.Context()
		if _, _, err := c.Read(ctx); err != nil { // consume setup
			return
		}
		serverMsg := serverMessage{ServerContent: &serverContent{
			OutputTranscription: &transcriptionText{Text: "Merhaba, nasıl yardımcı olabilirim?"},
		}}
		payload, _ := json.Marshal(serverMsg)
		c.Write(ctx, websocket.MessageText, payload)
		<-ctx.Done()
	}))
	defer srv.Close()
	original := SessionBaseURL
	SessionBaseURL = "ws" + strings.TrimPrefix(srv.URL, "http")
	defer func() { SessionBaseURL = original }()

	c := NewClient("g-key", "models/gemini-3.1-flash-live-preview", "", nil, nil)
	if err := c.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer c.Close()

	select {
	case ev := <-c.Events():
		if ev.Type != livemode.EventTranscript {
			t.Fatalf("expected a transcript event, got %s (err=%v)", ev.Type, ev.Err)
		}
		if ev.Transcript != "Merhaba, nasıl yardımcı olabilirim?" {
			t.Errorf("expected the transcript text to round-trip, got %q", ev.Transcript)
		}
		if ev.Role != livemode.RoleModel {
			t.Errorf("expected Role=model for an output transcript, got %q", ev.Role)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for the transcript event")
	}
}

// TestClient_EmptyInputTranscriptionEmitsNoEvent confirms an empty (but
// present) InputTranscription doesn't produce a spurious empty-string
// EventTranscript.
func TestClient_EmptyInputTranscriptionEmitsNoEvent(t *testing.T) {
	audioB64 := base64.StdEncoding.EncodeToString([]byte("reply-pcm-bytes"))
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer c.CloseNow()
		ctx := r.Context()
		if _, _, err := c.Read(ctx); err != nil { // consume setup
			return
		}
		serverMsg := serverMessage{ServerContent: &serverContent{
			InputTranscription: &transcriptionText{Text: ""},
			ModelTurn: &modelTurn{Parts: []serverPart{{InlineData: &inlineData{
				MimeType: "audio/pcm;rate=24000",
				Data:     audioB64,
			}}}},
		}}
		payload, _ := json.Marshal(serverMsg)
		c.Write(ctx, websocket.MessageText, payload)
		<-ctx.Done()
	}))
	defer srv.Close()
	original := SessionBaseURL
	SessionBaseURL = "ws" + strings.TrimPrefix(srv.URL, "http")
	defer func() { SessionBaseURL = original }()

	c := NewClient("g-key", "models/gemini-3.1-flash-live-preview", "", nil, nil)
	if err := c.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer c.Close()

	select {
	case ev := <-c.Events():
		if ev.Type != livemode.EventAudioOut {
			t.Errorf("expected the first (only) event to be audio_out, not a spurious empty transcript; got %s", ev.Type)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for the audio_out event")
	}
}

// TestClient_ReadsMessagesLargerThan32KB is a regression test for a real
// bug found via a real live connection: coder/websocket's default 32768-
// byte read limit silently closed the connection (StatusMessageTooBig) the
// first time a serverContent audio chunk's base64-encoded inlineData
// exceeded it — well within reach of a fraction of a second of 24kHz PCM.
// Sends a >40KB audio payload (comfortably over the old default) and
// confirms it round-trips whole, proving Start()'s SetReadLimit call is
// actually in effect rather than just present in the source.
func TestClient_ReadsMessagesLargerThan32KB(t *testing.T) {
	rawAudio := make([]byte, 40000)
	for i := range rawAudio {
		rawAudio[i] = byte(i % 256)
	}
	audioB64 := base64.StdEncoding.EncodeToString(rawAudio)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer c.CloseNow()
		ctx := r.Context()
		if _, _, err := c.Read(ctx); err != nil { // consume setup
			return
		}
		serverMsg := serverMessage{ServerContent: &serverContent{
			ModelTurn: &modelTurn{Parts: []serverPart{{InlineData: &inlineData{
				MimeType: "audio/pcm;rate=24000",
				Data:     audioB64,
			}}}},
		}}
		payload, _ := json.Marshal(serverMsg)
		c.Write(ctx, websocket.MessageText, payload)
		<-ctx.Done()
	}))
	defer srv.Close()
	original := SessionBaseURL
	SessionBaseURL = "ws" + strings.TrimPrefix(srv.URL, "http")
	defer func() { SessionBaseURL = original }()

	c := NewClient("g-key", "models/gemini-3.1-flash-live-preview", "", nil, nil)
	if err := c.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer c.Close()

	select {
	case ev := <-c.Events():
		if ev.Type != livemode.EventAudioOut {
			t.Fatalf("expected an audio_out event, got %s (err=%v) -- the old 32KB default read limit would surface this as EventError", ev.Type, ev.Err)
		}
		if len(ev.Audio) != len(rawAudio) {
			t.Errorf("expected %d bytes of audio, got %d", len(rawAudio), len(ev.Audio))
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for the audio_out event")
	}
}

func TestClient_SendsFunctionDeclarationsInSetup(t *testing.T) {
	f := newFakeLiveServer(t)
	original := SessionBaseURL
	SessionBaseURL = f.wsURL()
	defer func() { SessionBaseURL = original }()

	tools := []livemode.ToolSpec{livemode.DelegateToolSpec()}
	c := NewClient("g-key", "models/gemini-3.1-flash-live-preview", "", tools, nil)
	if err := c.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer c.Close()

	select {
	case setup := <-f.gotSetup:
		if len(setup.Tools) != 1 || len(setup.Tools[0].FunctionDeclarations) != 1 {
			t.Fatalf("expected exactly 1 functionDeclarations entry, got %+v", setup.Tools)
		}
		if setup.Tools[0].FunctionDeclarations[0].Name != livemode.DelegateToolName {
			t.Errorf("expected %s, got %q", livemode.DelegateToolName, setup.Tools[0].FunctionDeclarations[0].Name)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for setup message")
	}
}

// TestClient_ToolCallInvokesHandlerAndSendsToolResponse proves the full
// tool-call round trip: a fake Google Live server sends a toolCall message,
// the client's handleToolCall is invoked with the right name/args, and its
// result is sent back as a toolResponse the fake server can observe.
func TestClient_ToolCallInvokesHandlerAndSendsToolResponse(t *testing.T) {
	gotToolResponse := make(chan toolResponseMessage, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer c.CloseNow()
		ctx := r.Context()
		if _, _, err := c.Read(ctx); err != nil { // consume setup
			return
		}

		toolCallMsg := serverMessage{ToolCall: &toolCall{FunctionCalls: []functionCall{
			{ID: "call-1", Name: "delegate_to_main_model", Args: json.RawMessage(`{"instruction":"fix the bug"}`)},
		}}}
		payload, _ := json.Marshal(toolCallMsg)
		c.Write(ctx, websocket.MessageText, payload)

		for {
			_, data, err := c.Read(ctx)
			if err != nil {
				return
			}
			var msg clientMessage
			if err := json.Unmarshal(data, &msg); err == nil && msg.ToolResponse != nil {
				select {
				case gotToolResponse <- *msg.ToolResponse:
				default:
				}
			}
		}
	}))
	defer srv.Close()

	original := SessionBaseURL
	SessionBaseURL = "ws" + strings.TrimPrefix(srv.URL, "http")
	defer func() { SessionBaseURL = original }()

	var gotName string
	var gotArgs json.RawMessage
	handler := func(ctx context.Context, name string, args json.RawMessage) (string, error) {
		gotName = name
		gotArgs = args
		return "bug fixed", nil
	}

	c := NewClient("g-key", "models/gemini-3.1-flash-live-preview", "", []livemode.ToolSpec{livemode.DelegateToolSpec()}, handler)
	if err := c.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer c.Close()

	select {
	case resp := <-gotToolResponse:
		if len(resp.FunctionResponses) != 1 {
			t.Fatalf("expected 1 functionResponse, got %+v", resp.FunctionResponses)
		}
		fr := resp.FunctionResponses[0]
		if fr.ID != "call-1" || fr.Name != "delegate_to_main_model" {
			t.Errorf("unexpected functionResponse identity: %+v", fr)
		}
		if fr.Response["result"] != "bug fixed" {
			t.Errorf("expected result=%q, got %+v", "bug fixed", fr.Response)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for the toolResponse")
	}

	if gotName != "delegate_to_main_model" {
		t.Errorf("handler called with wrong name: %q", gotName)
	}
	if string(gotArgs) != `{"instruction":"fix the bug"}` {
		t.Errorf("handler called with wrong args: %s", gotArgs)
	}
}

func TestClient_ToolCallWithNilHandlerReportsNotAvailable(t *testing.T) {
	gotToolResponse := make(chan toolResponseMessage, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer c.CloseNow()
		ctx := r.Context()
		if _, _, err := c.Read(ctx); err != nil {
			return
		}
		payload, _ := json.Marshal(serverMessage{ToolCall: &toolCall{FunctionCalls: []functionCall{
			{ID: "call-1", Name: "delegate_to_main_model", Args: json.RawMessage(`{}`)},
		}}})
		c.Write(ctx, websocket.MessageText, payload)

		for {
			_, data, err := c.Read(ctx)
			if err != nil {
				return
			}
			var msg clientMessage
			if err := json.Unmarshal(data, &msg); err == nil && msg.ToolResponse != nil {
				select {
				case gotToolResponse <- *msg.ToolResponse:
				default:
				}
			}
		}
	}))
	defer srv.Close()

	original := SessionBaseURL
	SessionBaseURL = "ws" + strings.TrimPrefix(srv.URL, "http")
	defer func() { SessionBaseURL = original }()

	c := NewClient("g-key", "models/gemini-3.1-flash-live-preview", "", nil, nil)
	if err := c.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer c.Close()

	select {
	case resp := <-gotToolResponse:
		result, _ := resp.FunctionResponses[0].Response["result"].(string)
		if !strings.Contains(result, "not available") {
			t.Errorf("expected a 'not available' error result, got %q", result)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for the toolResponse")
	}
}

func TestClient_InjectContextSendsRealtimeInputText(t *testing.T) {
	gotText := make(chan string, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer c.CloseNow()
		ctx := r.Context()
		if _, _, err := c.Read(ctx); err != nil { // consume setup
			return
		}
		_, data, err := c.Read(ctx)
		if err != nil {
			return
		}
		var msg clientMessage
		if err := json.Unmarshal(data, &msg); err == nil && msg.RealtimeInput != nil {
			gotText <- msg.RealtimeInput.Text
		}
		<-ctx.Done()
	}))
	defer srv.Close()

	original := SessionBaseURL
	SessionBaseURL = "ws" + strings.TrimPrefix(srv.URL, "http")
	defer func() { SessionBaseURL = original }()

	c := NewClient("g-key", "models/gemini-3.1-flash-live-preview", "", nil, nil)
	if err := c.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer c.Close()

	if err := c.InjectContext("relevant memory: the user prefers dark mode"); err != nil {
		t.Fatalf("InjectContext: %v", err)
	}

	select {
	case text := <-gotText:
		if text != "relevant memory: the user prefers dark mode" {
			t.Errorf("unexpected injected text: %q", text)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for the injected realtimeInput.text message")
	}
}
