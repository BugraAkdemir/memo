// SPDX-License-Identifier: AGPL-3.0-or-later

package geminisub

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"memo/internal/provider"

	"golang.org/x/oauth2"
)

func TestNewProvider_RegisteredAndNeverFails(t *testing.T) {
	// The RegisterConstructor in init() must let provider.NewProvider build a
	// gemini-sub provider even with no token — a disconnected account cannot
	// break router construction.
	p, err := provider.NewProvider(provider.ProviderConfig{Type: provider.ProviderGeminiSub, Model: "gemini-2.5-pro"})
	if err != nil {
		t.Fatalf("provider.NewProvider(gemini-sub) = %v", err)
	}
	if p.Name() != provider.ProviderGeminiSub {
		t.Errorf("Name() = %q, want gemini-sub", p.Name())
	}
}

func TestProvider_NotConnected(t *testing.T) {
	p := &geminiSubProvider{mgr: newTestManager(t), client: &http.Client{}, streamCl: &http.Client{}}

	if _, err := p.ListModels(context.Background()); !errors.Is(err, ErrNotConnected) {
		t.Errorf("ListModels err = %v, want ErrNotConnected", err)
	}
	if _, err := p.ChatCompletion(context.Background(), provider.ChatRequest{}); !errors.Is(err, ErrNotConnected) {
		t.Errorf("ChatCompletion err = %v, want ErrNotConnected", err)
	}
	if _, err := p.ChatCompletionStream(context.Background(), provider.ChatRequest{}); !errors.Is(err, ErrNotConnected) {
		t.Errorf("ChatCompletionStream err = %v, want ErrNotConnected", err)
	}
}

func TestProvider_ResolveModel(t *testing.T) {
	p := &geminiSubProvider{model: "gemini-2.5-pro"}
	cases := map[string]string{
		"":                        "gemini-2.5-pro",
		"gemini-2.5-flash":        "gemini-2.5-flash",
		"models/gemini-2.5-flash": "gemini-2.5-flash",
		"gemini-sub/gemini-2.5-pro": "gemini-2.5-pro",
	}
	for in, want := range cases {
		if got := p.resolveModel(in); got != want {
			t.Errorf("resolveModel(%q) = %q, want %q", in, got, want)
		}
	}
}

// codeAssistServer stands in for cloudcode-pa: it answers the bootstrap and
// then generateContent / streamGenerateContent.
func codeAssistServer(t *testing.T, gen http.HandlerFunc) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.Header.Get("Authorization"), "Bearer ") {
			// bootstrap and generate both require the bearer token
			http.Error(w, "no bearer", http.StatusUnauthorized)
			return
		}
		switch {
		case strings.HasSuffix(r.URL.Path, ":loadCodeAssist"):
			_, _ = w.Write([]byte(`{"cloudaicompanionProject":"proj-1","currentTier":{"id":"pro"}}`))
		case strings.HasSuffix(r.URL.Path, ":generateContent"), strings.HasSuffix(r.URL.Path, ":streamGenerateContent"):
			gen(w, r)
		default:
			http.Error(w, "unexpected "+r.URL.Path, http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)
	t.Setenv(envEndpoint, srv.URL+"/v1internal")
}

func connectedProvider(t *testing.T) *geminiSubProvider {
	t.Helper()
	m := newTestManager(t)
	m.token = &oauth2.Token{AccessToken: "at-live", Expiry: time.Now().Add(time.Hour)}
	// Prevent the refreshing TokenSource from trying to hit Google: a token
	// with a far-future expiry and no refresh token is returned as-is.
	p, _ := NewProvider(provider.ProviderConfig{Type: provider.ProviderGeminiSub, Model: "gemini-2.5-pro"})
	gs := p.(*geminiSubProvider)
	gs.mgr = m
	return gs
}

func TestProvider_ChatCompletion(t *testing.T) {
	var gotBody, gotAuth string
	codeAssistServer(t, func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		gotAuth = r.Header.Get("Authorization")
		_, _ = w.Write([]byte(`{"response":{"candidates":[{"content":{"parts":[{"text":"pong"}]},"finishReason":"STOP"}],"usageMetadata":{"totalTokenCount":3}}}`))
	})

	p := connectedProvider(t)
	resp, err := p.ChatCompletion(context.Background(), provider.ChatRequest{
		Model:    "gemini-2.5-pro",
		Messages: []provider.Message{{Role: "user", Content: "ping"}},
	})
	if err != nil {
		t.Fatalf("ChatCompletion: %v", err)
	}
	if resp.Content != "pong" {
		t.Errorf("content = %q", resp.Content)
	}
	if gotAuth != "Bearer at-live" {
		t.Errorf("Authorization = %q, want Bearer at-live", gotAuth)
	}
	if !strings.Contains(gotBody, `"project":"proj-1"`) || !strings.Contains(gotBody, `"request":`) {
		t.Errorf("request not wrapped in the Code Assist envelope: %s", gotBody)
	}
}

func TestProvider_ChatCompletionStream(t *testing.T) {
	codeAssistServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fl, _ := w.(http.Flusher)
		for _, chunk := range []string{
			`data: {"response":{"candidates":[{"content":{"parts":[{"text":"he"}]},"finishReason":""}]}}`,
			`data: {"response":{"candidates":[{"content":{"parts":[{"text":"llo"}]},"finishReason":"STOP"}]}}`,
		} {
			_, _ = io.WriteString(w, chunk+"\n\n")
			if fl != nil {
				fl.Flush()
			}
		}
	})

	p := connectedProvider(t)
	ch, err := p.ChatCompletionStream(context.Background(), provider.ChatRequest{
		Model:    "gemini-2.5-pro",
		Messages: []provider.Message{{Role: "user", Content: "hi"}},
	})
	if err != nil {
		t.Fatalf("ChatCompletionStream: %v", err)
	}
	var got strings.Builder
	var sawDone bool
	for chunk := range ch {
		got.WriteString(chunk.Content)
		if chunk.Done {
			sawDone = true
		}
	}
	if got.String() != "hello" {
		t.Errorf("streamed content = %q, want hello", got.String())
	}
	if !sawDone {
		t.Error("stream never sent a Done chunk")
	}
}

func TestProvider_ChatCompletion_APIError(t *testing.T) {
	codeAssistServer(t, func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":{"message":"quota exceeded"}}`, http.StatusTooManyRequests)
	})
	p := connectedProvider(t)
	_, err := p.ChatCompletion(context.Background(), provider.ChatRequest{Messages: []provider.Message{{Role: "user", Content: "x"}}})
	if err == nil || !strings.Contains(err.Error(), "quota exceeded") {
		t.Errorf("err = %v, want it to carry the API error message", err)
	}
}
