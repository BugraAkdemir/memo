package webserver

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"memo/internal/livemode"
)

// TestHandleLiveModeEngineModels_ErrorIsJSON proves the fix for a real bug
// reported live: ListLiveModeEngineModels failing (bad API key, rate
// limit, network error) used to reach the client as a plain-text
// http.Error body, which Dio hands back as a raw String — FriendlyError.
// describeGeneric's _messageFromResponseBody only extracts a message from
// a JSON object, so the actual reason was silently replaced with the
// generic "Something went wrong" fallback. The handler must always answer
// 200 with a {"status":...} JSON body (mirroring handleProviderModels),
// so api_client.dart's listLiveModeEngineModels can surface the real
// error message.
func TestHandleLiveModeEngineModels_ErrorIsJSON(t *testing.T) {
	srv := &Server{fullBridge: &swarmStubBridge{
		listLiveModeEngineModels: func(ctx context.Context, t livemode.EngineType, apiKey string) ([]livemode.ModelInfo, error) {
			return nil, errors.New("livemode google: models status 400: API key not valid")
		},
	}}

	body, _ := json.Marshal(map[string]string{"type": "google_live", "api_key": "bad-key"})
	req := httptest.NewRequest(http.MethodPost, "/api/livemode/engines/models", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	srv.handleLiveModeEngineModels(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (errors are reported in the JSON body, not the HTTP status)", rr.Code)
	}
	var got map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatalf("response body is not JSON: %v (body: %s)", err, rr.Body.String())
	}
	if got["status"] != "error" {
		t.Errorf(`status = %v, want "error"`, got["status"])
	}
	if got["error"] != "livemode google: models status 400: API key not valid" {
		t.Errorf("error = %v, want the real underlying error message", got["error"])
	}
}

func TestHandleLiveModeEngineModels_SuccessIsJSON(t *testing.T) {
	srv := &Server{fullBridge: &swarmStubBridge{
		listLiveModeEngineModels: func(ctx context.Context, t livemode.EngineType, apiKey string) ([]livemode.ModelInfo, error) {
			return []livemode.ModelInfo{{ID: "models/gemini-3.1-flash-live-preview"}}, nil
		},
	}}

	body, _ := json.Marshal(map[string]string{"type": "google_live", "api_key": "real-key"})
	req := httptest.NewRequest(http.MethodPost, "/api/livemode/engines/models", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	srv.handleLiveModeEngineModels(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	var got map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatalf("response body is not JSON: %v", err)
	}
	if got["status"] != "ok" {
		t.Errorf(`status = %v, want "ok"`, got["status"])
	}
	models, _ := got["models"].([]any)
	if len(models) != 1 {
		t.Errorf("models = %v, want one model", got["models"])
	}
}
