package provider

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestGeminiProvider_ChatCompletion_SendsToolsAndParsesFunctionCall is the
// regression test for the second provider found with zero tool-calling
// support: Memo's "Google Gemini" type never sent req.Tools and never
// parsed a functionCall part back out, so agent mode silently degraded to
// plain chat here too, identically to claude.go's bug (see
// claude_toolcalling_test.go's doc comment for how this was found).
func TestGeminiProvider_ChatCompletion_SendsToolsAndParsesFunctionCall(t *testing.T) {
	var gotBody geminiRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(body, &gotBody); err != nil {
			t.Fatalf("could not decode outgoing request: %v", err)
		}
		json.NewEncoder(w).Encode(geminiResponse{
			Candidates: []geminiCandidate{{
				FinishReason: "STOP",
				Content: geminiContent{
					Role: "model",
					Parts: []geminiPart{
						{Text: "Klasörü listeliyorum."},
						{FunctionCall: &geminiFunctionCall{ID: "call-1", Name: "list_directory", Args: json.RawMessage(`{"path":"."}`)}},
					},
				},
			}},
		})
	}))
	defer srv.Close()

	p, err := newGeminiProvider(ProviderConfig{BaseURL: srv.URL, Model: "gemini-2.0-flash", APIKey: "test-key"})
	if err != nil {
		t.Fatalf("newGeminiProvider() error = %v", err)
	}
	resp, err := p.ChatCompletion(context.Background(), ChatRequest{
		Messages: []Message{TextMessage("user", "list files")},
		Tools: []ToolDefinition{{
			Type: "function",
			Function: ToolFunction{
				Name:        "list_directory",
				Description: "List a directory",
				Parameters:  json.RawMessage(`{"type":"object","properties":{"path":{"type":"string"}}}`),
			},
		}},
	})
	if err != nil {
		t.Fatalf("ChatCompletion() error = %v", err)
	}

	// Outgoing: exactly one tools[] entry, holding the one declared function.
	if len(gotBody.Tools) != 1 {
		t.Fatalf("outgoing Tools = %+v, want exactly 1 entry (Gemini wants ALL functions in ONE entry)", gotBody.Tools)
	}
	if len(gotBody.Tools[0].FunctionDeclarations) != 1 || gotBody.Tools[0].FunctionDeclarations[0].Name != "list_directory" {
		t.Errorf("outgoing FunctionDeclarations = %+v, want [list_directory]", gotBody.Tools[0].FunctionDeclarations)
	}
	if string(gotBody.Tools[0].FunctionDeclarations[0].Parameters) != `{"type":"object","properties":{"path":{"type":"string"}}}` {
		t.Errorf("outgoing parameters = %s, want the schema passed straight through", gotBody.Tools[0].FunctionDeclarations[0].Parameters)
	}

	// Incoming: functionCall part parsed into ToolCalls, text part still
	// concatenated into Content.
	if resp.Content != "Klasörü listeliyorum." {
		t.Errorf("resp.Content = %q, want the text part", resp.Content)
	}
	if len(resp.ToolCalls) != 1 {
		t.Fatalf("resp.ToolCalls = %+v, want exactly 1", resp.ToolCalls)
	}
	tc := resp.ToolCalls[0]
	if tc.ID != "call-1" || tc.Function.Name != "list_directory" {
		t.Errorf("resp.ToolCalls[0] = %+v, want id=call-1 name=list_directory", tc)
	}
	if string(tc.Function.Arguments) != `{"path":"."}` {
		t.Errorf("resp.ToolCalls[0].Function.Arguments = %s, want the functionCall args verbatim", tc.Function.Arguments)
	}
}

// TestGeminiProvider_ChatCompletion_RoundTripsToolResult mirrors claude.go's
// equivalent: the assistant ("model") turn's functionCall must be echoed
// back, and the tool's result arrives as a functionResponse part on a
// following "user" turn — never a bare "tool"-role message (Gemini has no
// such role).
func TestGeminiProvider_ChatCompletion_RoundTripsToolResult(t *testing.T) {
	var gotBody geminiRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &gotBody)
		json.NewEncoder(w).Encode(geminiResponse{
			Candidates: []geminiCandidate{{FinishReason: "STOP", Content: geminiContent{Parts: []geminiPart{{Text: "Tamamdır."}}}}},
		})
	}))
	defer srv.Close()

	p, err := newGeminiProvider(ProviderConfig{BaseURL: srv.URL, Model: "gemini-2.0-flash", APIKey: "test-key"})
	if err != nil {
		t.Fatalf("newGeminiProvider() error = %v", err)
	}
	_, err = p.ChatCompletion(context.Background(), ChatRequest{
		Messages: []Message{
			TextMessage("user", "list files"),
			{
				Role: "assistant",
				ToolCalls: []ToolCall{
					{ID: "call-1", Type: "function", Function: ToolCallFunction{Name: "list_directory", Arguments: json.RawMessage(`{"path":"."}`)}},
				},
			},
			{Role: "tool", ToolCallID: "call-1", Content: "a.txt\nb.txt"},
		},
	})
	if err != nil {
		t.Fatalf("ChatCompletion() error = %v", err)
	}

	if len(gotBody.Contents) != 3 {
		t.Fatalf("outgoing Contents = %+v, want 3 (user text, model functionCall, user functionResponse)", gotBody.Contents)
	}

	var sawFunctionCall, sawFunctionResponse bool
	for _, c := range gotBody.Contents {
		for _, part := range c.Parts {
			if part.FunctionCall != nil {
				sawFunctionCall = true
				if c.Role != "model" {
					t.Errorf("functionCall part found on role %q, want model", c.Role)
				}
				if part.FunctionCall.ID != "call-1" || part.FunctionCall.Name != "list_directory" {
					t.Errorf("echoed functionCall = %+v, want id=call-1 name=list_directory", part.FunctionCall)
				}
			}
			if part.FunctionResponse != nil {
				sawFunctionResponse = true
				if c.Role != "user" {
					t.Errorf("functionResponse part found on role %q, want user", c.Role)
				}
				if part.FunctionResponse.ID != "call-1" {
					t.Errorf("functionResponse.id = %q, want call-1", part.FunctionResponse.ID)
				}
				if part.FunctionResponse.Name != "list_directory" {
					t.Errorf("functionResponse.name = %q, want list_directory (recovered from the matching functionCall)", part.FunctionResponse.Name)
				}
				var decoded map[string]string
				json.Unmarshal(part.FunctionResponse.Response, &decoded)
				if decoded["result"] != "a.txt\nb.txt" {
					t.Errorf("functionResponse.response = %s, want the tool's raw output wrapped as JSON", part.FunctionResponse.Response)
				}
			}
		}
	}
	if !sawFunctionCall {
		t.Error("no functionCall part found in the outgoing request")
	}
	if !sawFunctionResponse {
		t.Error("no functionResponse part found in the outgoing request")
	}
}

// TestGeminiProvider_ChatCompletion_NoToolsOmitsToolsField mirrors the
// Claude test: a plain chat call must not send an empty/present "tools" key.
func TestGeminiProvider_ChatCompletion_NoToolsOmitsToolsField(t *testing.T) {
	var raw map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &raw)
		json.NewEncoder(w).Encode(geminiResponse{Candidates: []geminiCandidate{{Content: geminiContent{Parts: []geminiPart{{Text: "hi"}}}}}})
	}))
	defer srv.Close()

	p, err := newGeminiProvider(ProviderConfig{BaseURL: srv.URL, Model: "gemini-2.0-flash", APIKey: "test-key"})
	if err != nil {
		t.Fatalf("newGeminiProvider() error = %v", err)
	}
	_, err = p.ChatCompletion(context.Background(), ChatRequest{Messages: []Message{TextMessage("user", "hi")}})
	if err != nil {
		t.Fatalf("ChatCompletion() error = %v", err)
	}
	if _, present := raw["tools"]; present {
		t.Errorf("outgoing request has a \"tools\" key with no tools configured, want it omitted entirely: %v", raw["tools"])
	}
}
