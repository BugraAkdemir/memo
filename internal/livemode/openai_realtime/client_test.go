package openai_realtime

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

type fakeRealtimeServer struct {
	srv           *httptest.Server
	gotAuth       chan string
	gotModelQuery chan string
	gotUpdate     chan sessionUpdateEvent
	gotAudioIn    chan string
}

func newFakeRealtimeServer(t *testing.T) *fakeRealtimeServer {
	t.Helper()
	f := &fakeRealtimeServer{
		gotAuth:       make(chan string, 1),
		gotModelQuery: make(chan string, 1),
		gotUpdate:     make(chan sessionUpdateEvent, 1),
		gotAudioIn:    make(chan string, 8),
	}
	f.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.gotAuth <- r.Header.Get("Authorization")
		f.gotModelQuery <- r.URL.Query().Get("model")

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
		var first sessionUpdateEvent
		if err := json.Unmarshal(data, &first); err != nil || first.Type != "session.update" {
			t.Errorf("expected the first client message to be session.update, got %q", data)
			return
		}
		f.gotUpdate <- first

		for {
			_, data, err := c.Read(ctx)
			if err != nil {
				return
			}
			var ev inputAudioBufferAppendEvent
			if err := json.Unmarshal(data, &ev); err != nil {
				continue
			}
			if ev.Type == "input_audio_buffer.append" {
				select {
				case f.gotAudioIn <- ev.Audio:
				default:
				}
			}
		}
	}))
	t.Cleanup(f.srv.Close)
	return f
}

func (f *fakeRealtimeServer) wsURL() string {
	return "ws" + strings.TrimPrefix(f.srv.URL, "http")
}

func TestClient_SendsAuthHeaderAndModelQueryParam(t *testing.T) {
	f := newFakeRealtimeServer(t)
	original := SessionBaseURL
	SessionBaseURL = f.wsURL()
	defer func() { SessionBaseURL = original }()

	c := NewClient("oa-key", "gpt-realtime-2.1", "", nil, nil)
	if err := c.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer c.Close()

	select {
	case auth := <-f.gotAuth:
		if auth != "Bearer oa-key" {
			t.Errorf("expected Bearer oa-key, got %q", auth)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for the Authorization header")
	}
	select {
	case model := <-f.gotModelQuery:
		if model != "gpt-realtime-2.1" {
			t.Errorf("expected model query param gpt-realtime-2.1, got %q", model)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for the model query param")
	}
}

func TestClient_SendsSessionUpdateOnStart(t *testing.T) {
	f := newFakeRealtimeServer(t)
	original := SessionBaseURL
	SessionBaseURL = f.wsURL()
	defer func() { SessionBaseURL = original }()

	c := NewClient("oa-key", "gpt-realtime-2.1", "You are Memo's live voice.", nil, nil)
	if err := c.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer c.Close()

	select {
	case update := <-f.gotUpdate:
		if update.Session.Model != "gpt-realtime-2.1" {
			t.Errorf("unexpected model in session.update: %q", update.Session.Model)
		}
		if update.Session.Instructions != "You are Memo's live voice." {
			t.Errorf("unexpected instructions: %q", update.Session.Instructions)
		}
		if len(update.Session.OutputModalities) != 1 || update.Session.OutputModalities[0] != "audio" {
			t.Errorf("expected output_modalities=[audio], got %v", update.Session.OutputModalities)
		}
		if update.Session.Audio == nil || update.Session.Audio.Input == nil || update.Session.Audio.Input.Format.Rate != inputSampleRateHz {
			t.Errorf("expected audio.input.format.rate=%d, got %+v", inputSampleRateHz, update.Session.Audio)
		}
		if update.Session.Audio.Input.Transcription == nil || update.Session.Audio.Input.Transcription.Model != "whisper-1" {
			t.Errorf("expected audio.input.transcription={model:whisper-1} to always be enabled, got %+v", update.Session.Audio.Input.Transcription)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for session.update on the server side")
	}
}

func TestClient_SendAudioForwardsBase64EncodedPCM(t *testing.T) {
	f := newFakeRealtimeServer(t)
	original := SessionBaseURL
	SessionBaseURL = f.wsURL()
	defer func() { SessionBaseURL = original }()

	c := NewClient("oa-key", "gpt-realtime-2.1", "", nil, nil)
	if err := c.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer c.Close()
	<-f.gotUpdate // wait past the session.update handshake

	if err := c.SendAudio([]byte("raw-pcm-bytes")); err != nil {
		t.Fatalf("SendAudio: %v", err)
	}

	select {
	case b64 := <-f.gotAudioIn:
		decoded, err := base64.StdEncoding.DecodeString(b64)
		if err != nil {
			t.Fatalf("decode: %v", err)
		}
		if string(decoded) != "raw-pcm-bytes" {
			t.Errorf("expected round-tripped PCM bytes, got %q", decoded)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for audio chunk on the server side")
	}
}

func TestClient_EmitsAudioOutFromResponseOutputAudioDelta(t *testing.T) {
	audioB64 := base64.StdEncoding.EncodeToString([]byte("reply-pcm-bytes"))
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer c.CloseNow()
		ctx := r.Context()
		if _, _, err := c.Read(ctx); err != nil { // consume session.update
			return
		}
		payload, _ := json.Marshal(serverEvent{Type: serverEventAudioDelta, Delta: audioB64})
		c.Write(ctx, websocket.MessageText, payload)
		<-ctx.Done()
	}))
	defer srv.Close()

	original := SessionBaseURL
	SessionBaseURL = "ws" + strings.TrimPrefix(srv.URL, "http")
	defer func() { SessionBaseURL = original }()

	c := NewClient("oa-key", "gpt-realtime-2.1", "", nil, nil)
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

func TestClient_IgnoresUnrelatedServerEventTypes(t *testing.T) {
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
		// session.created / input_audio_buffer.speech_started style events
		// must not produce an audio_out (or any) event.
		payload, _ := json.Marshal(serverEvent{Type: "session.created"})
		c.Write(ctx, websocket.MessageText, payload)
		<-ctx.Done()
	}))
	defer srv.Close()

	original := SessionBaseURL
	SessionBaseURL = "ws" + strings.TrimPrefix(srv.URL, "http")
	defer func() { SessionBaseURL = original }()

	c := NewClient("oa-key", "gpt-realtime-2.1", "", nil, nil)
	if err := c.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer c.Close()

	select {
	case ev := <-c.Events():
		t.Fatalf("expected no event for an unrelated server event type, got %+v", ev)
	case <-time.After(300 * time.Millisecond):
		// expected: nothing arrives
	}
}

// TestClient_EmitsTranscriptFromInputTranscriptionCompleted confirms
// readLoop parses conversation.item.input_audio_transcription.completed
// (the user-speech transcript) into an EventTranscript.
func TestClient_EmitsTranscriptFromInputTranscriptionCompleted(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer c.CloseNow()
		ctx := r.Context()
		if _, _, err := c.Read(ctx); err != nil { // consume session.update
			return
		}
		payload, _ := json.Marshal(serverEvent{
			Type:       serverEventInputTranscriptionComplete,
			Transcript: "kapıyı kilitle",
		})
		c.Write(ctx, websocket.MessageText, payload)
		<-ctx.Done()
	}))
	defer srv.Close()

	original := SessionBaseURL
	SessionBaseURL = "ws" + strings.TrimPrefix(srv.URL, "http")
	defer func() { SessionBaseURL = original }()

	c := NewClient("oa-key", "gpt-realtime-2.1", "", nil, nil)
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

// TestClient_EmitsTranscriptFromOutputTranscriptDone confirms readLoop also
// surfaces the model's own spoken reply (as text) as an EventTranscript
// with Role=model — added for the Live Mode UI's "show the live
// conversation as a normal chat" follow-up (see
// docs/plans/PLAN_live_mode_v2.md), distinct from the user's own Role=user
// transcript above.
func TestClient_EmitsTranscriptFromOutputTranscriptDone(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer c.CloseNow()
		ctx := r.Context()
		if _, _, err := c.Read(ctx); err != nil { // consume session.update
			return
		}
		payload, _ := json.Marshal(serverEvent{
			Type:       serverEventOutputTranscriptDone,
			Transcript: "Merhaba, nasıl yardımcı olabilirim?",
		})
		c.Write(ctx, websocket.MessageText, payload)
		<-ctx.Done()
	}))
	defer srv.Close()

	original := SessionBaseURL
	SessionBaseURL = "ws" + strings.TrimPrefix(srv.URL, "http")
	defer func() { SessionBaseURL = original }()

	c := NewClient("oa-key", "gpt-realtime-2.1", "", nil, nil)
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

// TestClient_EmptyTranscriptionCompletedEmitsNoEvent confirms an empty
// transcript field doesn't produce a spurious empty-string EventTranscript.
func TestClient_EmptyTranscriptionCompletedEmitsNoEvent(t *testing.T) {
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
		payload, _ := json.Marshal(serverEvent{Type: serverEventInputTranscriptionComplete, Transcript: ""})
		c.Write(ctx, websocket.MessageText, payload)
		<-ctx.Done()
	}))
	defer srv.Close()

	original := SessionBaseURL
	SessionBaseURL = "ws" + strings.TrimPrefix(srv.URL, "http")
	defer func() { SessionBaseURL = original }()

	c := NewClient("oa-key", "gpt-realtime-2.1", "", nil, nil)
	if err := c.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer c.Close()

	select {
	case ev := <-c.Events():
		t.Fatalf("expected no event for an empty transcript, got %+v", ev)
	case <-time.After(300 * time.Millisecond):
		// expected: nothing arrives
	}
}

// TestClient_SendsVoiceInSessionUpdate confirms the optional voice argument
// (requested by the user alongside the full-screen UI work — "let us
// change the voice") reaches session.audio.output.voice.
func TestClient_SendsVoiceInSessionUpdate(t *testing.T) {
	f := newFakeRealtimeServer(t)
	original := SessionBaseURL
	SessionBaseURL = f.wsURL()
	defer func() { SessionBaseURL = original }()

	c := NewClient("oa-key", "gpt-realtime-2.1", "", nil, nil, "marin")
	if err := c.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer c.Close()

	select {
	case update := <-f.gotUpdate:
		if update.Session.Audio == nil || update.Session.Audio.Output == nil || update.Session.Audio.Output.Voice != "marin" {
			t.Errorf("expected audio.output.voice=%q, got %+v", "marin", update.Session.Audio)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for session.update on the server side")
	}
}

// TestClient_OmitsVoiceWhenNoneGiven confirms the default (no-voice-
// argument) call shape — every other test in this file — leaves
// audio.output.voice entirely unset rather than sending an empty string,
// so the provider's own default voice is used unmodified.
func TestClient_OmitsVoiceWhenNoneGiven(t *testing.T) {
	f := newFakeRealtimeServer(t)
	original := SessionBaseURL
	SessionBaseURL = f.wsURL()
	defer func() { SessionBaseURL = original }()

	c := NewClient("oa-key", "gpt-realtime-2.1", "", nil, nil)
	if err := c.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer c.Close()

	select {
	case update := <-f.gotUpdate:
		if update.Session.Audio != nil && update.Session.Audio.Output != nil && update.Session.Audio.Output.Voice != "" {
			t.Errorf("expected no voice override when none was given, got %q", update.Session.Audio.Output.Voice)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for session.update on the server side")
	}
}

func TestClient_SendsToolsInSessionUpdate(t *testing.T) {
	f := newFakeRealtimeServer(t)
	original := SessionBaseURL
	SessionBaseURL = f.wsURL()
	defer func() { SessionBaseURL = original }()

	tools := []livemode.ToolSpec{livemode.DelegateToolSpec()}
	c := NewClient("oa-key", "gpt-realtime-2.1", "", tools, nil)
	if err := c.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer c.Close()

	select {
	case update := <-f.gotUpdate:
		if len(update.Session.Tools) != 1 {
			t.Fatalf("expected exactly 1 tool, got %+v", update.Session.Tools)
		}
		if update.Session.Tools[0].Name != livemode.DelegateToolName || update.Session.Tools[0].Type != "function" {
			t.Errorf("unexpected tool entry: %+v", update.Session.Tools[0])
		}
		if update.Session.ToolChoice != "auto" {
			t.Errorf("expected tool_choice=auto, got %q", update.Session.ToolChoice)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for session.update")
	}
}

// TestClient_ToolCallInvokesHandlerAndSendsFunctionCallOutput proves the
// full tool-call round trip: a fake server sends
// response.function_call_arguments.done, the client's handleToolCall is
// invoked with the right name/args, and its result is sent back as
// conversation.item.create (function_call_output) followed by
// response.create.
func TestClient_ToolCallInvokesHandlerAndSendsFunctionCallOutput(t *testing.T) {
	gotOutput := make(chan functionCallOutputItem, 1)
	gotResponseCreate := make(chan struct{}, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer c.CloseNow()
		ctx := r.Context()
		if _, _, err := c.Read(ctx); err != nil { // consume session.update
			return
		}

		payload, _ := json.Marshal(serverEvent{
			Type:      serverEventFunctionCallArgsDone,
			CallID:    "call-1",
			Name:      "delegate_to_main_model",
			Arguments: `{"instruction":"fix the bug"}`,
		})
		c.Write(ctx, websocket.MessageText, payload)

		for {
			_, data, err := c.Read(ctx)
			if err != nil {
				return
			}
			var itemEv conversationItemCreateEvent
			if err := json.Unmarshal(data, &itemEv); err == nil && itemEv.Type == "conversation.item.create" {
				select {
				case gotOutput <- itemEv.Item:
				default:
				}
				continue
			}
			var rc responseCreateEvent
			if err := json.Unmarshal(data, &rc); err == nil && rc.Type == "response.create" {
				select {
				case gotResponseCreate <- struct{}{}:
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

	c := NewClient("oa-key", "gpt-realtime-2.1", "", []livemode.ToolSpec{livemode.DelegateToolSpec()}, handler)
	if err := c.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer c.Close()

	select {
	case item := <-gotOutput:
		if item.CallID != "call-1" || item.Type != "function_call_output" {
			t.Errorf("unexpected function_call_output identity: %+v", item)
		}
		if item.Output != "bug fixed" {
			t.Errorf("expected output=%q, got %q", "bug fixed", item.Output)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for function_call_output")
	}
	select {
	case <-gotResponseCreate:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for response.create")
	}

	if gotName != "delegate_to_main_model" {
		t.Errorf("handler called with wrong name: %q", gotName)
	}
	if string(gotArgs) != `{"instruction":"fix the bug"}` {
		t.Errorf("handler called with wrong args: %s", gotArgs)
	}
}

func TestClient_ToolCallWithNilHandlerReportsNotAvailable(t *testing.T) {
	gotOutput := make(chan functionCallOutputItem, 1)
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
		payload, _ := json.Marshal(serverEvent{
			Type:      serverEventFunctionCallArgsDone,
			CallID:    "call-1",
			Name:      "delegate_to_main_model",
			Arguments: `{}`,
		})
		c.Write(ctx, websocket.MessageText, payload)

		for {
			_, data, err := c.Read(ctx)
			if err != nil {
				return
			}
			var itemEv conversationItemCreateEvent
			if err := json.Unmarshal(data, &itemEv); err == nil && itemEv.Type == "conversation.item.create" {
				select {
				case gotOutput <- itemEv.Item:
				default:
				}
			}
		}
	}))
	defer srv.Close()

	original := SessionBaseURL
	SessionBaseURL = "ws" + strings.TrimPrefix(srv.URL, "http")
	defer func() { SessionBaseURL = original }()

	c := NewClient("oa-key", "gpt-realtime-2.1", "", nil, nil)
	if err := c.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer c.Close()

	select {
	case item := <-gotOutput:
		if !strings.Contains(item.Output, "not available") {
			t.Errorf("expected a 'not available' error output, got %q", item.Output)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for function_call_output")
	}
}

func TestClient_InjectContextSendsSystemMessageItem(t *testing.T) {
	gotItem := make(chan messageItem, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer c.CloseNow()
		ctx := r.Context()
		if _, _, err := c.Read(ctx); err != nil { // consume session.update
			return
		}
		_, data, err := c.Read(ctx)
		if err != nil {
			return
		}
		var msg conversationItemCreateMessageEvent
		if err := json.Unmarshal(data, &msg); err == nil && msg.Type == "conversation.item.create" {
			gotItem <- msg.Item
		}
		<-ctx.Done()
	}))
	defer srv.Close()

	original := SessionBaseURL
	SessionBaseURL = "ws" + strings.TrimPrefix(srv.URL, "http")
	defer func() { SessionBaseURL = original }()

	c := NewClient("oa-key", "gpt-realtime-2.1", "", nil, nil)
	if err := c.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer c.Close()

	if err := c.InjectContext("relevant memory: the user prefers dark mode"); err != nil {
		t.Fatalf("InjectContext: %v", err)
	}

	select {
	case item := <-gotItem:
		if item.Type != "message" || item.Role != "system" {
			t.Errorf("unexpected item identity: %+v", item)
		}
		if len(item.Content) != 1 || item.Content[0].Text != "relevant memory: the user prefers dark mode" {
			t.Errorf("unexpected item content: %+v", item.Content)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for the injected conversation.item.create message")
	}
}
