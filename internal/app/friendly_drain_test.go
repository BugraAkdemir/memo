package app

import (
	"strings"
	"testing"

	"memo/internal/api"
	"memo/internal/config"
)

func errChan(msg string) <-chan api.StreamChunk {
	ch := make(chan api.StreamChunk, 2)
	ch <- api.StreamChunk{Content: "partial"}
	ch <- api.StreamChunk{Error: msg, Done: true}
	close(ch)
	return ch
}

const rawProvider400 = `⚠️ [custom] status 400: Your request was rejected by the safety system. request ID 9f3a, safety_violations=[sexual]`

// A chat surface or a non-streaming endpoint hands the error text to a person.
func TestDrains_TurnAProviderFailureIntoAnExplanation(t *testing.T) {
	a := &App{cfg: &config.AppConfig{}}

	for name, got := range map[string]string{
		"self-chat turn":       a.drainSelfChatTurn(errChan(rawProvider400), false, nil, nil, nil).Text,
		"self-chat reply":      a.drainSelfChatReply(errChan(rawProvider400), false, nil, nil, nil),
		"non-streaming reply":  drainToReplyWith(errChan(rawProvider400), a.FriendlyError),
		"live delegated reply": a.drainLiveDelegatedReply(errChan(rawProvider400), false, nil, nil, nil),
	} {
		if strings.Contains(got, "request ID") || strings.Contains(got, "safety_violations") || strings.Contains(got, "status 400") {
			t.Errorf("%s leaked the provider's raw text: %q", name, got)
		}
		if !strings.Contains(got, "(HTTP 400)") || !strings.Contains(got, "safety filter") {
			t.Errorf("%s = %q, want a sentence about the safety filter tagged with the status", name, got)
		}
	}
}

func TestDrains_CommonStatusCodesReadAsSentences(t *testing.T) {
	a := &App{cfg: &config.AppConfig{}}
	cases := map[string]string{
		"⚠️ [openrouter] status 404: model not found":                       "not found",
		"⚠️ [custom] status 401: invalid api key":                           "verify who you are",
		"⚠️ [custom] status 500: internal error":                            "can't answer right now",
		"⚠️ [custom] status 429: slow down":                                 "too many requests",
		"⚠️ all providers failed: dial tcp 1.2.3.4:443: connection refused": "Could not reach",
	}
	for raw, want := range cases {
		got := a.drainSelfChatTurn(errChan(raw), false, nil, nil, nil).Text
		if !strings.Contains(got, want) {
			t.Errorf("%q -> %q, want it to say %q", raw, got, want)
		}
	}
}

// Text that is not a provider failure is shown as it is, never swallowed.
func TestDrains_UnrecognisedErrorTextIsKept(t *testing.T) {
	a := &App{cfg: &config.AppConfig{}}
	if got := a.drainSelfChatTurn(errChan("⚠️ something only Memo knows"), false, nil, nil, nil).Text; got != "⚠️ something only Memo knows" {
		t.Errorf("got %q", got)
	}
	// And the plain drain keeps its old contract.
	if got := drainToReply(errChan("boom")); got != "boom" {
		t.Errorf("drainToReply = %q", got)
	}
}

func TestDrainSelfChatTurn_CollectsDrawnPicturesInOrder(t *testing.T) {
	a := &App{cfg: &config.AppConfig{}}
	ch := make(chan api.StreamChunk, 8)
	ch <- api.StreamChunk{Content: "here you go"}
	ch <- api.StreamChunk{FinishReason: imageGenerationMarker, Content: "/data/generated-images/a.png"}
	ch <- api.StreamChunk{FinishReason: imageGenerationMarker, Content: "/data/generated-images/b.png"}
	ch <- api.StreamChunk{FinishReason: imageGenerationMarker, Content: ""}
	ch <- api.StreamChunk{Done: true, FinishReason: "stop"}
	close(ch)

	got := a.drainSelfChatTurn(ch, false, nil, nil, nil)
	if got.Text != "here you go" || len(got.Images) != 2 || got.Images[0] != "/data/generated-images/a.png" || got.Images[1] != "/data/generated-images/b.png" {
		t.Errorf("got %+v", got)
	}
}
