package provider

import "testing"

// TestExtractErrorMessage_UnwrapsTheRealMessage is a regression test:
// parseError used to surface a failed request's entire raw JSON error body
// straight through — nested braces, "param":null, duplicate "type"/"code"
// fields — which then flowed through Router's "all providers failed: %w"
// wrapping into the chat UI verbatim. A non-technical user has no way to
// tell what actually went wrong from that. This is the exact error body
// OpenCode Zen returned for a 400 on an agent (tool-using) request.
func TestExtractErrorMessage_UnwrapsTheRealMessage(t *testing.T) {
	body := []byte(`{"error":{"message":"Error from provider (Console): Upstream request failed","type":"invalid_request_error","param":null,"code":"invalid_request_error"}}`)

	got := ExtractErrorMessage(body)
	want := "Error from provider (Console): Upstream request failed"
	if got != want {
		t.Errorf("ExtractErrorMessage() = %q, want %q", got, want)
	}
}

// TestExtractErrorMessage_FallsBackToRawBody covers a body that isn't in
// the {"error": {"message": ...}} shape (e.g. a plain-text error, an HTML
// error page from a misconfigured base URL, or a provider using a
// different error shape entirely) — the raw body must still come through,
// rather than being silently dropped.
func TestExtractErrorMessage_FallsBackToRawBody(t *testing.T) {
	body := []byte("upstream is down")
	if got := ExtractErrorMessage(body); got != "upstream is down" {
		t.Errorf("ExtractErrorMessage() = %q, want the raw body unchanged", got)
	}
}

// TestExtractErrorMessage_FallsBackWhenMessageEmpty covers valid JSON in the
// expected shape but with an empty message field — still not useful, so it
// should fall back to the raw body rather than returning "".
func TestExtractErrorMessage_FallsBackWhenMessageEmpty(t *testing.T) {
	body := []byte(`{"error":{"message":"","type":"server_error"}}`)
	got := ExtractErrorMessage(body)
	if got != string(body) {
		t.Errorf("ExtractErrorMessage() = %q, want the raw body since message was empty", got)
	}
}
