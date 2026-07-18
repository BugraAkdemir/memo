package webserver

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// The Flutter-specific handlers all guard on s.fullBridge (or, for the
// calendar/learning trio, a type-assertion on s.bridge) before touching it.
// This locks in that every one of them degrades to a clear error response
// instead of a nil-pointer panic when the app runs without a FullBridge wired
// in — e.g. a webserver constructed before Startup() finishes, or a
// deliberately bridge-less embedding. None of this was covered before: every
// handler here previously sat at 0% coverage.
func TestHandlers_NoFullBridge(t *testing.T) {
	s := newMockServer(&mockBridge{})

	tests := []struct {
		name       string
		method     string
		path       string
		handler    func(http.ResponseWriter, *http.Request)
		wantStatus int
	}{
		{"LocalModels", http.MethodGet, "/api/models/local", s.handleLocalModels, http.StatusNotImplemented},
		{"Events", http.MethodGet, "/api/events", s.handleEvents, http.StatusNotFound},
		{"Version", http.MethodGet, "/api/version", s.handleVersion, http.StatusOK},
		{"VersionCheck", http.MethodGet, "/api/version/check", s.handleVersionCheck, http.StatusMethodNotAllowed},
		{"GPU", http.MethodGet, "/api/gpu", s.handleGPU, http.StatusOK},
		{"MemoryEnabled", http.MethodGet, "/api/memory/enabled", s.handleMemoryEnabled, http.StatusNotImplemented},
		{"ModelStatus", http.MethodGet, "/api/models/status", s.handleModelStatus, http.StatusOK},
		{"EmbeddingStatus", http.MethodGet, "/api/models/embedding/status", s.handleEmbeddingStatus, http.StatusOK},
		{"SystemPrompt", http.MethodGet, "/api/system-prompt", s.handleSystemPrompt, http.StatusNotImplemented},
		{"ResetSystemPrompt", http.MethodPost, "/api/system-prompt/reset", s.handleResetSystemPrompt, http.StatusMethodNotAllowed},
		{"MinimalMode", http.MethodGet, "/api/system-prompt/minimal-mode", s.handleMinimalMode, http.StatusNotImplemented},
		{"IncognitoPrompt", http.MethodGet, "/api/incognito-prompt", s.handleIncognitoPrompt, http.StatusNotImplemented},
		{"MemoryFiles", http.MethodGet, "/api/memory/files", s.handleMemoryFiles, http.StatusNotImplemented},
		{"MemoryClear", http.MethodPost, "/api/memory/clear", s.handleMemoryClear, http.StatusMethodNotAllowed},
		{"MemorySettings", http.MethodGet, "/api/memory/settings", s.handleMemorySettings, http.StatusNotImplemented},
		{"MemoryStats", http.MethodGet, "/api/memory/stats", s.handleMemoryStats, http.StatusMethodNotAllowed},
		{"MemoryDebugSearch", http.MethodGet, "/api/memory/debug-search?q=x", s.handleMemoryDebugSearch, http.StatusMethodNotAllowed},
		{"ModelSearch", http.MethodPost, "/api/models/search", s.handleModelSearch, http.StatusMethodNotAllowed},
		{"ModelFiles", http.MethodGet, "/api/models/files", s.handleModelFiles, http.StatusNotImplemented},
		{"LlamaConfigGet", http.MethodGet, "/api/models/config", s.handleLlamaConfigGet, http.StatusNotImplemented},
		{"LlamaCheck", http.MethodGet, "/api/models/llama/check", s.handleLlamaCheck, http.StatusOK},
		{"RemoteAccess", http.MethodGet, "/api/remote-access", s.handleRemoteAccess, http.StatusNotImplemented},
		{"SyncSettings", http.MethodGet, "/api/sync/settings", s.handleSyncSettings, http.StatusNotImplemented},
		{"SyncAuth", http.MethodGet, "/api/sync/auth", s.handleSyncAuth, http.StatusNotImplemented},
		{"SyncAccount", http.MethodGet, "/api/sync/account", s.handleSyncAccount, http.StatusOK},
		{"SyncTrigger", http.MethodPost, "/api/sync/trigger", s.handleSyncTrigger, http.StatusMethodNotAllowed},
		{"SyncPull", http.MethodPost, "/api/sync/pull", s.handleSyncPull, http.StatusMethodNotAllowed},
		{"SyncNow", http.MethodPost, "/api/sync/now", s.handleSyncNow, http.StatusMethodNotAllowed},
		{"SyncDisconnect", http.MethodPost, "/api/sync/disconnect", s.handleSyncDisconnect, http.StatusMethodNotAllowed},
		{"Providers", http.MethodGet, "/api/providers", s.handleProviders, http.StatusNotImplemented},
		{"ProviderTest", http.MethodPost, "/api/providers/test", s.handleProviderTest, http.StatusMethodNotAllowed},
		{"ActiveProvider", http.MethodGet, "/api/providers/active", s.handleActiveProvider, http.StatusServiceUnavailable},
		{"Image", http.MethodGet, "/api/image?path=x", s.handleImage, http.StatusNotImplemented},
		{"ExportChat", http.MethodGet, "/api/chat/export", s.handleExportChat, http.StatusNotImplemented},
		{"GenerateTitle", http.MethodGet, "/api/chat/title", s.handleGenerateTitle, http.StatusNotImplemented},
		{"AgentUndo", http.MethodPost, "/api/agent/undo", s.handleAgentUndo, http.StatusMethodNotAllowed},
		{"Export", http.MethodGet, "/api/export", s.handleExport, http.StatusMethodNotAllowed},
		{"Import", http.MethodPost, "/api/import", s.handleImport, http.StatusMethodNotAllowed},
		{"Wipe", http.MethodPost, "/api/wipe", s.handleWipe, http.StatusMethodNotAllowed},
		{"CalendarEvents", http.MethodGet, "/api/calendar/events", s.handleCalendarEvents, http.StatusNotImplemented},
		{"CalendarSettings", http.MethodGet, "/api/calendar/settings", s.handleCalendarSettings, http.StatusNotImplemented},
		{"LearningSettings", http.MethodGet, "/api/learning/settings", s.handleLearningSettings, http.StatusNotImplemented},
		{"DevGatewayConfig", http.MethodGet, "/api/dev-gateway/config", s.handleDevGatewayConfig, http.StatusServiceUnavailable},
		{"DevGatewayModels", http.MethodGet, "/api/dev-gateway/models", s.handleDevGatewayModels, http.StatusMethodNotAllowed},
		{"AnthropicMessages", http.MethodPost, "/v1/messages", s.handleAnthropicMessages, http.StatusServiceUnavailable},
		{"DevGatewayLogs", http.MethodGet, "/api/dev-gateway/logs", s.handleDevGatewayLogs, http.StatusMethodNotAllowed},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			w := httptest.NewRecorder()

			tt.handler(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("%s %s: got status %d, want %d (body: %q)",
					tt.method, tt.path, w.Code, tt.wantStatus, w.Body.String())
			}
		})
	}
}
