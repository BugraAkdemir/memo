package webserver

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"memo/internal/agentcli"
	"memo/internal/api"
	"memo/internal/config"
	"memo/internal/logx"
	"memo/internal/orchestra"
	"memo/internal/provider"
	"memo/internal/shutdown"
	"memo/internal/tts"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// ─── Chat Streaming (SSE) ───────────────────────────────────────

func (s *Server) handleSendStream(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost || s.fullBridge == nil {
		http.Error(w, "not available", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Message string `json:"message"`
		// ChatID is optional (PLAN_chatid_refactor.md Faz 4) — a caller that
		// knows which chat it's actually talking to (Flutter's currently
		// open chat, internal/replcli's own session) should send it so the
		// message lands there regardless of which chat the backend considers
		// globally "active" at the moment the request is handled. Omitted
		// for backward compatibility with older clients, which fall back to
		// the previous implicit-active-chat behavior.
		ChatID string `json:"chat_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}

	// Set headers for SSE
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Transfer-Encoding", "chunked")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	ctx := r.Context()
	var ch <-chan api.StreamChunk
	if req.ChatID != "" {
		ch = s.fullBridge.SendMessageStreamTo(ctx, req.ChatID, req.Message)
	} else {
		ch = s.fullBridge.SendMessageStream(ctx, req.Message)
	}
	streamSSE(ctx, w, flusher, ch)
}

// streamSSE drains ch, writing each chunk to w as an SSE `data: {...}` line
// until ch closes or a Done:true chunk is written, preferring delivery of an
// already-ready chunk over honoring ctx cancellation. A plain
// `select { case <-ctx.Done(): return; case chunk, ok := <-ch: ... }` lets
// Go's random tie-breaking between simultaneously-ready select cases silently
// drop a chunk — including the final Done:true one — if ctx becomes Done at
// the exact moment ch also has a value ready. The Flutter client then never
// sees a `"done":true` line, so its "sending" UI state (the stop-button icon)
// stays stuck forever even though the backend finished the turn cleanly (see
// the matching fix on trySend/recvChunk in internal/app/llm.go and
// forwardStream in internal/app/chat.go — this is the outermost, last-hop
// layer of the same bug).
func streamSSE(ctx context.Context, w http.ResponseWriter, flusher http.Flusher, ch <-chan api.StreamChunk) {
	for {
		select {
		case chunk, ok := <-ch:
			if !ok {
				return
			}
			if writeSSEChunk(w, flusher, chunk) {
				return
			}
			continue
		default:
		}
		select {
		case <-ctx.Done():
			return
		case chunk, ok := <-ch:
			if !ok {
				return
			}
			if writeSSEChunk(w, flusher, chunk) {
				return
			}
		}
	}
}

func writeSSEChunk(w http.ResponseWriter, flusher http.Flusher, chunk api.StreamChunk) bool {
	data, err := json.Marshal(chunk)
	if err != nil {
		return false
	}
	fmt.Fprintf(w, "data: %s\n\n", string(data))
	flusher.Flush()
	return chunk.Done
}

func (s *Server) handleSendFileStream(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost || s.fullBridge == nil {
		http.Error(w, "not available", http.StatusMethodNotAllowed)
		return
	}

	err := r.ParseMultipartForm(50 << 20)
	if err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}

	msg := r.FormValue("message")

	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "missing file", http.StatusBadRequest)
		return
	}
	defer file.Close()

	tmpFile, err := os.CreateTemp("", "memo_web_*_"+header.Filename)
	if err != nil {
		http.Error(w, "tmp error", http.StatusInternalServerError)
		return
	}
	tmpFilePath := tmpFile.Name()
	defer os.Remove(tmpFilePath)
	defer tmpFile.Close()

	if _, err := io.Copy(tmpFile, file); err != nil {
		http.Error(w, "copy error", http.StatusInternalServerError)
		return
	}
	tmpFile.Close()

	// detectIsImageFile sniffs actual file content rather than trusting the
	// client-supplied Content-Type header (the non-streaming handleSendFile
	// in server.go already does this) — a header alone lets a client label
	// any file "image/png" and have it routed into the vision pipeline
	// regardless of its actual content.
	isImage := detectIsImageFile(tmpFilePath, header.Filename)

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Transfer-Encoding", "chunked")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	ctx := r.Context()
	var ch <-chan api.StreamChunk
	if isImage {
		ch = s.fullBridge.SendMessageWithImageStream(ctx, msg, tmpFilePath)
	} else {
		ch = s.fullBridge.SendMessageWithFileStream(ctx, msg, tmpFilePath)
	}

	streamSSE(ctx, w, flusher, ch)
}

// ─── Backup / Restore (.memo) ─────────────────────────────────────────────────

func (s *Server) handleExport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet || s.fullBridge == nil {
		http.Error(w, "GET only", http.StatusMethodNotAllowed)
		return
	}
	includeModels := r.URL.Query().Get("include_models") == "true"
	data, err := s.fullBridge.ExportData(includeModels)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", "attachment; filename=memo_backup.memo")
	w.Write(data)
}

func (s *Server) handleImport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost || s.fullBridge == nil {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}
	defer r.Body.Close()
	data, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "read error", http.StatusBadRequest)
		return
	}
	if err := s.fullBridge.ImportData(data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]string{"status": "ok"})
}

func (s *Server) handleWipe(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost || s.fullBridge == nil {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}
	if err := s.fullBridge.WipeAllData(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]string{"status": "ok"})
}

// ─── CLI / Uninstall ─────────────────────────────────────────────

func (s *Server) handleCLIRemove(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost || s.fullBridge == nil {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}
	if err := s.fullBridge.RemoveCLI(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]string{"status": "ok"})
}

func (s *Server) handleCLIReinstall(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost || s.fullBridge == nil {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}
	if err := s.fullBridge.ReinstallCLI(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]string{"status": "ok"})
}

func (s *Server) handleUninstall(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost || s.fullBridge == nil {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		KeepMemory bool `json:"keep_memory"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	if err := s.fullBridge.UninstallMemo(req.KeepMemory); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]string{"status": "ok"})
}

// ─── System Prompt ──────────────────────────────────────────────

func (s *Server) handleSystemPrompt(w http.ResponseWriter, r *http.Request) {
	if s.fullBridge == nil {
		http.Error(w, "not available", http.StatusNotImplemented)
		return
	}
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, map[string]string{"prompt": s.fullBridge.GetSystemPrompt()})
	case http.MethodPut:
		var req struct {
			Prompt string `json:"prompt"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "bad json", http.StatusBadRequest)
			return
		}
		if err := s.fullBridge.SetSystemPrompt(req.Prompt); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]string{"ok": "true"})
	default:
		http.Error(w, "GET or PUT", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleResetSystemPrompt(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost || s.fullBridge == nil {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}
	if err := s.fullBridge.ResetSystemPrompt(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]string{"ok": "true"})
}

// ─── Minimal Mode ───────────────────────────────────────────────
//
// When on, identity/persona/mood/web-search prompt injection is disabled
// entirely — only memory context (if separately enabled) still reaches
// the model. For a tight local-model context budget.

func (s *Server) handleMinimalMode(w http.ResponseWriter, r *http.Request) {
	if s.fullBridge == nil {
		http.Error(w, "not available", http.StatusNotImplemented)
		return
	}
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, map[string]bool{"enabled": s.fullBridge.GetMinimalMode()})
	case http.MethodPut:
		var req struct {
			Enabled bool `json:"enabled"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "bad json", http.StatusBadRequest)
			return
		}
		if err := s.fullBridge.SetMinimalMode(req.Enabled); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]string{"ok": "true"})
	default:
		http.Error(w, "GET or PUT", http.StatusMethodNotAllowed)
	}
}

// ─── UI Language ────────────────────────────────────────────────
//
// The backend has no display language of its own — this exists purely so a
// second client with no SharedPreferences of its own (the terminal REPL,
// internal/replcli) can follow whatever language the Flutter GUI's own
// locale toggle (frontend/lib/core/l10n.dart) is set to, instead of always
// defaulting to Turkish. The GUI writes this whenever its own toggle
// changes; it never needs to read it back (SharedPreferences is already its
// source of truth).

func (s *Server) handleUILanguage(w http.ResponseWriter, r *http.Request) {
	if s.fullBridge == nil {
		http.Error(w, "not available", http.StatusNotImplemented)
		return
	}
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, map[string]string{"language": s.fullBridge.GetUILanguage()})
	case http.MethodPut:
		var req struct {
			Language string `json:"language"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "bad json", http.StatusBadRequest)
			return
		}
		if req.Language != "tr" && req.Language != "en" {
			http.Error(w, `language must be "tr" or "en"`, http.StatusBadRequest)
			return
		}
		if err := s.fullBridge.SetUILanguage(req.Language); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]string{"ok": "true"})
	default:
		http.Error(w, "GET or PUT", http.StatusMethodNotAllowed)
	}
}

// handleMinimalModeOverrides gets/sets the four granular Minimal Mode
// overrides (Settings' "keep X on even in Minimal Mode" dropdown) — each
// only has any effect while Minimal Mode itself is on, see
// app.MinimalModeOverrides's doc comment.
func (s *Server) handleMinimalModeOverrides(w http.ResponseWriter, r *http.Request) {
	if s.fullBridge == nil {
		http.Error(w, "not available", http.StatusNotImplemented)
		return
	}
	switch r.Method {
	case http.MethodGet:
		persona, capabilities, passive, proactive := s.fullBridge.GetMinimalModeOverrides()
		writeJSON(w, map[string]bool{
			"keep_persona":      persona,
			"keep_capabilities": capabilities,
			"keep_passive":      passive,
			"keep_proactive":    proactive,
		})
	case http.MethodPut:
		var req struct {
			KeepPersona      bool `json:"keep_persona"`
			KeepCapabilities bool `json:"keep_capabilities"`
			KeepPassive      bool `json:"keep_passive"`
			KeepProactive    bool `json:"keep_proactive"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "bad json", http.StatusBadRequest)
			return
		}
		if err := s.fullBridge.SetMinimalModeOverrides(req.KeepPersona, req.KeepCapabilities, req.KeepPassive, req.KeepProactive); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]string{"ok": "true"})
	default:
		http.Error(w, "GET or PUT", http.StatusMethodNotAllowed)
	}
}

// ─── Incognito Prompt ───────────────────────────────────────────

func (s *Server) handleIncognitoPrompt(w http.ResponseWriter, r *http.Request) {
	if s.fullBridge == nil {
		http.Error(w, "not available", http.StatusNotImplemented)
		return
	}
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, map[string]string{"prompt": s.fullBridge.GetIncognitoPrompt()})
	case http.MethodPut:
		var req struct {
			Prompt string `json:"prompt"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "bad json", http.StatusBadRequest)
			return
		}
		if err := s.fullBridge.SetIncognitoPrompt(req.Prompt); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]string{"ok": "true"})
	default:
		http.Error(w, "GET or PUT", http.StatusMethodNotAllowed)
	}
}

// ─── Memory ─────────────────────────────────────────────────────

func (s *Server) handleMemoryFiles(w http.ResponseWriter, r *http.Request) {
	if s.fullBridge == nil {
		http.Error(w, "not available", http.StatusNotImplemented)
		return
	}
	switch r.Method {
	case http.MethodGet:
		files := s.fullBridge.ListMemoryFiles()
		if files == nil {
			writeJSON(w, []struct{}{})
			return
		}
		writeJSON(w, files)
	case http.MethodDelete:
		var req struct {
			Path string `json:"path"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "bad json", http.StatusBadRequest)
			return
		}
		if err := s.fullBridge.DeleteMemoryFile(req.Path); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]string{"ok": "true"})
	default:
		http.Error(w, "GET or DELETE", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleAgentUndo(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost || s.fullBridge == nil {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}
	if err := s.fullBridge.UndoLastAgentEdit(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]string{"ok": "true"})
}

func (s *Server) handleMemoryClear(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost || s.fullBridge == nil {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}
	if err := s.fullBridge.ClearAllMemory(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]string{"ok": "true"})
}

func (s *Server) handleOnboarding(w http.ResponseWriter, r *http.Request) {
	if s.fullBridge == nil {
		http.Error(w, "not available", http.StatusNotImplemented)
		return
	}
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, map[string]bool{"completed": s.fullBridge.GetOnboardingComplete()})
	case http.MethodPut:
		var req struct {
			Completed bool `json:"completed"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "bad json", http.StatusBadRequest)
			return
		}
		if err := s.fullBridge.SetOnboardingComplete(req.Completed); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]string{"ok": "true"})
	default:
		http.Error(w, "GET or PUT", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleMemoryEnabled(w http.ResponseWriter, r *http.Request) {
	if s.fullBridge == nil {
		http.Error(w, "not available", http.StatusNotImplemented)
		return
	}
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, map[string]bool{"enabled": s.fullBridge.GetMemoryEnabled()})
	case http.MethodPut:
		var req struct {
			Enabled bool `json:"enabled"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "bad json", http.StatusBadRequest)
			return
		}
		if err := s.fullBridge.SetMemoryEnabled(req.Enabled); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]string{"ok": "true"})
	default:
		http.Error(w, "GET or PUT", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleMemorySettings(w http.ResponseWriter, r *http.Request) {
	if s.fullBridge == nil {
		http.Error(w, "not available", http.StatusNotImplemented)
		return
	}
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, s.fullBridge.GetMemorySettings())
	case http.MethodPut:
		var req struct {
			TopK          int     `json:"top_k"`
			MinSimilarity float32 `json:"min_similarity"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "bad json", http.StatusBadRequest)
			return
		}
		if err := s.fullBridge.UpdateMemorySettings(req.TopK, req.MinSimilarity); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		writeJSON(w, map[string]string{"ok": "true"})
	default:
		http.Error(w, "GET or PUT", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleMemoryDreamSettings(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut || s.fullBridge == nil {
		http.Error(w, "PUT only", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Enabled             bool `json:"enabled"`
		InitialDelayMinutes int  `json:"initial_delay_minutes"`
		IntervalHours       int  `json:"interval_hours"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	if err := s.fullBridge.SetMemoryDreamSettings(req.Enabled, req.InitialDelayMinutes, req.IntervalHours); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(w, map[string]string{"ok": "true"})
}

func (s *Server) handleMemoryDreamRun(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost || s.fullBridge == nil {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}
	before, after, ran, err := s.fullBridge.RunDreamNow(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]any{
		"ran":    ran,
		"before": before,
		"after":  after,
	})
}

func (s *Server) handleMemoryDebugSearch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet || s.fullBridge == nil {
		http.Error(w, "GET only", http.StatusMethodNotAllowed)
		return
	}
	query := r.URL.Query().Get("q")
	if query == "" {
		http.Error(w, "q parameter required", http.StatusBadRequest)
		return
	}
	results := s.fullBridge.DebugMemorySearch(query)
	if results == nil {
		writeJSON(w, []struct{}{})
		return
	}
	writeJSON(w, results)
}

// ─── Version & Image ────────────────────────────────────────────

func (s *Server) handleVersion(w http.ResponseWriter, r *http.Request) {
	if s.fullBridge == nil {
		writeJSON(w, map[string]string{"version": "unknown"})
		return
	}
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, map[string]string{"version": s.fullBridge.GetVersion()})
	default:
		http.Error(w, "GET only", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleVersionCheck(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet || s.fullBridge == nil {
		http.Error(w, "not available", http.StatusMethodNotAllowed)
		return
	}
	latest, err := s.fullBridge.CheckLatestVersion()
	if err != nil {
		// Return current version and the error for diagnostics
		writeJSON(w, map[string]interface{}{
			"current": s.fullBridge.GetVersion(),
			"latest":  "",
			"error":   err.Error(),
		})
		return
	}
	writeJSON(w, map[string]interface{}{
		"current": s.fullBridge.GetVersion(),
		"latest":  latest,
		"error":   nil,
	})
}

func (s *Server) handleImage(w http.ResponseWriter, r *http.Request) {
	if s.fullBridge == nil {
		http.Error(w, "not available", http.StatusNotImplemented)
		return
	}
	path := r.URL.Query().Get("path")
	if path == "" {
		http.Error(w, "missing path", http.StatusBadRequest)
		return
	}

	// Layer 1: Basic path sanitization (URL-encoded bypass'a karşı decode et)
	decoded, err := url.QueryUnescape(path)
	if err != nil {
		http.Error(w, "bad path", http.StatusBadRequest)
		return
	}
	if strings.Contains(decoded, "..") {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	if filepath.IsAbs(decoded) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	cleaned := filepath.Clean(decoded)
	if cleaned == "." || cleaned == ".." {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	allowed := false
	for _, prefix := range []string{"data/images", "data/avatars", "data/attachments"} {
		if cleaned == prefix || strings.HasPrefix(cleaned, prefix+"/") {
			allowed = true
			break
		}
	}
	if !allowed {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	b64 := s.fullBridge.GetImageBase64(cleaned)
	if b64 == "" {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	writeJSON(w, map[string]string{"data": b64})
}

// ─── Chat Export & Title ────────────────────────────────────────

func (s *Server) handleExportChat(w http.ResponseWriter, r *http.Request) {
	if s.fullBridge == nil {
		http.Error(w, "not available", http.StatusNotImplemented)
		return
	}
	md := s.fullBridge.ExportChat()
	writeJSON(w, map[string]string{"markdown": md})
}

func (s *Server) handleGenerateTitle(w http.ResponseWriter, r *http.Request) {
	if s.fullBridge == nil {
		http.Error(w, "not available", http.StatusNotImplemented)
		return
	}
	title := s.fullBridge.GenerateChatTitle()
	writeJSON(w, map[string]string{"title": title})
}

// ─── Remote Access ──────────────────────────────────────────────

func (s *Server) handleChatCLIProvider(w http.ResponseWriter, r *http.Request) {
	if s.fullBridge == nil {
		http.Error(w, "not available", http.StatusNotImplemented)
		return
	}
	switch r.Method {
	case http.MethodGet:
		id := r.URL.Query().Get("id")
		writeJSON(w, map[string]string{"cli_provider": s.fullBridge.GetChatCLIProvider(id)})
	case http.MethodPost:
		var req struct {
			ID          string `json:"id"`
			CLIProvider string `json:"cli_provider"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "bad json", http.StatusBadRequest)
			return
		}
		if err := s.fullBridge.SetChatCLIProvider(req.ID, req.CLIProvider); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]string{"ok": "true"})
	default:
		http.Error(w, "GET or POST", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleChatCLIWorkdir(w http.ResponseWriter, r *http.Request) {
	if s.fullBridge == nil {
		http.Error(w, "not available", http.StatusNotImplemented)
		return
	}
	switch r.Method {
	case http.MethodGet:
		id := r.URL.Query().Get("id")
		writeJSON(w, map[string]string{"workdir": s.fullBridge.GetChatCLIWorkdir(id)})
	case http.MethodPost:
		var req struct {
			ID      string `json:"id"`
			Workdir string `json:"workdir"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "bad json", http.StatusBadRequest)
			return
		}
		if err := s.fullBridge.SetChatCLIWorkdir(req.ID, req.Workdir); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]string{"ok": "true"})
	default:
		http.Error(w, "GET or POST", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleChatCLIModel(w http.ResponseWriter, r *http.Request) {
	if s.fullBridge == nil {
		http.Error(w, "not available", http.StatusNotImplemented)
		return
	}
	switch r.Method {
	case http.MethodGet:
		id := r.URL.Query().Get("id")
		writeJSON(w, map[string]string{"model": s.fullBridge.GetChatCLIModel(id)})
	case http.MethodPost:
		var req struct {
			ID    string `json:"id"`
			Model string `json:"model"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "bad json", http.StatusBadRequest)
			return
		}
		if err := s.fullBridge.SetChatCLIModel(req.ID, req.Model); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]string{"ok": "true"})
	default:
		http.Error(w, "GET or POST", http.StatusMethodNotAllowed)
	}
}

// handleCLIModelOptions serves the model ids a CLI-backed chat can switch to
// — the top bar's model picker when a chat is in CLI mode.
func (s *Server) handleCLIModelOptions(w http.ResponseWriter, r *http.Request) {
	if s.fullBridge == nil {
		http.Error(w, "not available", http.StatusNotImplemented)
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "GET only", http.StatusMethodNotAllowed)
		return
	}
	cliType := r.URL.Query().Get("type")
	if cliType == "" {
		http.Error(w, "missing type", http.StatusBadRequest)
		return
	}
	models := s.fullBridge.ListCLIModels(cliType)
	if models == nil {
		models = []string{}
	}
	writeJSON(w, map[string][]string{"models": models})
}

func (s *Server) handleSendCLIStream(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost || s.fullBridge == nil {
		http.Error(w, "not available", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		ChatID  string `json:"chat_id"`
		Message string `json:"message"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Transfer-Encoding", "chunked")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	ctx := r.Context()
	ch := s.fullBridge.SendCLIMessageStream(ctx, req.ChatID, req.Message)
	streamSSE(ctx, w, flusher, ch)
}

func (s *Server) handleCLIRunning(w http.ResponseWriter, r *http.Request) {
	if s.fullBridge == nil {
		http.Error(w, "not available", http.StatusNotImplemented)
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "GET only", http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, map[string][]string{"chat_ids": s.fullBridge.GetRunningCLIChats()})
}

func (s *Server) handleFileMentions(w http.ResponseWriter, r *http.Request) {
	if s.fullBridge == nil {
		http.Error(w, "not available", http.StatusNotImplemented)
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "GET only", http.StatusMethodNotAllowed)
		return
	}
	root := r.URL.Query().Get("root")
	query := r.URL.Query().Get("query")
	writeJSON(w, map[string][]string{"files": s.fullBridge.ListProjectFiles(root, query)})
}

// handleFileBrowse implements GET /api/files/browse?path=... — the backend
// half of the in-app server-side file browser (Faz 5.1 follow-up, see
// App.BrowseServerPath's doc comment). Lists path's immediate children;
// path="" starts at the server's home directory.
func (s *Server) handleFileBrowse(w http.ResponseWriter, r *http.Request) {
	if s.fullBridge == nil {
		http.Error(w, "not available", http.StatusNotImplemented)
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "GET only", http.StatusMethodNotAllowed)
		return
	}
	result, err := s.fullBridge.BrowseServerPath(r.URL.Query().Get("path"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(w, result)
}

// handleCLICommands serves the slash commands a CLI-backed chat can use, for
// the composer's "/" dropdown. chat_id is optional — without it only
// user-level and built-in commands are found, since project-level ones live
// under the chat's own working directory.
func (s *Server) handleCLICommands(w http.ResponseWriter, r *http.Request) {
	if s.fullBridge == nil {
		http.Error(w, "not available", http.StatusNotImplemented)
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "GET only", http.StatusMethodNotAllowed)
		return
	}
	cliType := r.URL.Query().Get("type")
	if cliType == "" {
		http.Error(w, "missing type", http.StatusBadRequest)
		return
	}
	cmds := s.fullBridge.ListCLICommands(cliType, r.URL.Query().Get("chat_id"))
	if cmds == nil {
		cmds = []agentcli.Command{}
	}
	writeJSON(w, map[string][]agentcli.Command{"commands": cmds})
}

func (s *Server) handleCLIStatus(w http.ResponseWriter, r *http.Request) {
	if s.fullBridge == nil {
		http.Error(w, "not available", http.StatusNotImplemented)
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "GET only", http.StatusMethodNotAllowed)
		return
	}
	cliType := r.URL.Query().Get("type")
	if cliType == "" {
		http.Error(w, "missing type", http.StatusBadRequest)
		return
	}
	writeJSON(w, s.fullBridge.GetCLIStatus(cliType))
}

func (s *Server) handleRemoteAccess(w http.ResponseWriter, r *http.Request) {
	if s.fullBridge == nil {
		http.Error(w, "not available", http.StatusNotImplemented)
		return
	}
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, s.fullBridge.GetRemoteAccessStatus())
	case http.MethodPut:
		if !s.callerIsAdmin(r) {
			http.Error(w, "forbidden: admin only", http.StatusForbidden)
			return
		}
		var req struct {
			Enabled        *bool  `json:"enabled"`
			Port           int    `json:"port"`
			NgrokMode      bool   `json:"ngrok_mode"`
			NgrokToken     string `json:"ngrok_token"`
			NgrokAutoStart *bool  `json:"ngrok_auto_start"`
			// Tailscale tunnel
			TunnelMode        string `json:"tunnel_mode"`
			TailscaleKey      string `json:"tailscale_key"`
			TailscaleHostname string `json:"tailscale_hostname"`
			TailscaleFunnel   bool   `json:"tailscale_funnel"`
			// Beta features toggle
			Beta *bool `json:"beta"`
			// Auth mode (Faz 2, yapacam.md) — omit AuthMode entirely to
			// leave the current mode/credentials untouched (e.g. a PUT that
			// only toggles Enabled/Port shouldn't accidentally reset auth).
			AuthMode string `json:"auth_mode"`
			Username string `json:"username"`
			Password string `json:"password"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "bad json", http.StatusBadRequest)
			return
		}
		if req.AuthMode != "" {
			if err := s.fullBridge.SetRemoteAuthConfig(req.AuthMode, req.Username, req.Password); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
		}
		if req.Beta != nil {
			if err := s.fullBridge.SetBeta(*req.Beta); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
		}
		if req.NgrokAutoStart != nil {
			s.fullBridge.SetNgrokAutoStart(*req.NgrokAutoStart)
		}
		if req.TunnelMode == "tailscale" {
			enabled := req.Enabled == nil || *req.Enabled
			if err := s.fullBridge.SetTailscaleMode(enabled, req.TailscaleKey, req.TailscaleHostname, req.TailscaleFunnel, req.Port); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
		} else if req.Enabled != nil {
			if req.NgrokMode {
				if err := s.fullBridge.SetNgrokMode(*req.Enabled, req.Port, req.NgrokToken); err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}
			} else {
				if err := s.fullBridge.SetRemoteAccess(*req.Enabled, req.Port); err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}
			}
		}
		// Return the full status (including the token) rather than a bare
		// {"ok": true} — this PUT is exactly the request that may just have
		// switched the listener from 127.0.0.1 (unauthenticated) to 0.0.0.0
		// (token-gated, see remoteAuthMiddleware). It's the caller's one
		// chance to learn the token from an already-authorized request; a
		// follow-up GET would otherwise 401 with no way to ever obtain it.
		writeJSON(w, s.fullBridge.GetRemoteAccessStatus())
	default:
		http.Error(w, "GET or PUT", http.StatusMethodNotAllowed)
	}
}

// ─── Sync ───────────────────────────────────────────────────────

func (s *Server) handleSyncAuth(w http.ResponseWriter, r *http.Request) {
	if s.fullBridge == nil {
		http.Error(w, "not available", http.StatusNotImplemented)
		return
	}
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, map[string]bool{"authenticated": s.fullBridge.CheckAuth()})
	case http.MethodPost:
		url, err := s.fullBridge.StartSyncAuth()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]string{"url": url})
	default:
		http.Error(w, "GET or POST", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleSyncAccount(w http.ResponseWriter, r *http.Request) {
	if s.fullBridge == nil {
		writeJSON(w, map[string]interface{}{"authenticated": false})
		return
	}
	writeJSON(w, s.fullBridge.GetSyncAccount())
}

func (s *Server) handleSyncSettings(w http.ResponseWriter, r *http.Request) {
	if s.fullBridge == nil {
		http.Error(w, "not available", http.StatusNotImplemented)
		return
	}
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, s.fullBridge.GetSyncSettings())
	case http.MethodPut:
		var req struct {
			Enabled          bool   `json:"enabled"`
			ClientID         string `json:"client_id"`
			ClientSecret     string `json:"client_secret"`
			Passphrase       string `json:"passphrase"`
			TokenPath        string `json:"token_path"`
			IntervalMessages int    `json:"interval_messages"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "bad json", http.StatusBadRequest)
			return
		}
		if err := s.fullBridge.UpdateSyncSettings(req.Enabled, req.ClientID, req.ClientSecret, req.Passphrase, req.TokenPath, req.IntervalMessages); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]string{"ok": "true"})
	default:
		http.Error(w, "GET or PUT", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleSyncTrigger(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost || s.fullBridge == nil {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}
	s.fullBridge.TriggerSync()
	writeJSON(w, map[string]string{"ok": "true"})
}

func (s *Server) handleSyncPull(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost || s.fullBridge == nil {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}
	s.fullBridge.PullSync()
	writeJSON(w, map[string]string{"ok": "true"})
}

func (s *Server) handleSyncNow(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost || s.fullBridge == nil {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}
	s.fullBridge.SyncNow()
	writeJSON(w, map[string]string{"ok": "true"})
}

func (s *Server) handleSyncDisconnect(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost || s.fullBridge == nil {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}
	if err := s.fullBridge.DisconnectSync(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]string{"ok": "true"})
}

func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "GET only", http.StatusMethodNotAllowed)
		return
	}
	if s.fullBridge == nil {
		http.Error(w, "not available", http.StatusNotFound)
		return
	}
	evs := s.fullBridge.GetEvents()
	writeJSON(w, evs)
}

// ─── Models ─────────────────────────────────────────────────────

func (s *Server) handleLocalModels(w http.ResponseWriter, r *http.Request) {
	if s.fullBridge == nil {
		http.Error(w, "not available", http.StatusNotImplemented)
		return
	}
	switch r.Method {
	case http.MethodGet:
		models := s.fullBridge.ListLocalModels()
		if models == nil {
			writeJSON(w, []struct{}{})
			return
		}
		writeJSON(w, models)
	case http.MethodDelete:
		var req struct {
			Path string `json:"path"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "bad json", http.StatusBadRequest)
			return
		}
		if err := s.fullBridge.DeleteLocalModel(req.Path); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]string{"ok": "true"})
	default:
		http.Error(w, "GET or DELETE", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleModelImport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost || s.fullBridge == nil {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Path string `json:"path"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	if err := s.fullBridge.ImportLocalModel(req.Path); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]string{"ok": "true"})
}

func (s *Server) handleModelStart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost || s.fullBridge == nil {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Path      string `json:"path"`
		CtxSize   int    `json:"ctx_size"`
		Port      int    `json:"port"`
		GPULayers int    `json:"gpu_layers"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	if err := s.fullBridge.StartLocalModel(req.Path, req.CtxSize, req.Port, req.GPULayers); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]string{"ok": "true"})
}

func (s *Server) handleModelStop(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost || s.fullBridge == nil {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}
	if err := s.fullBridge.StopLocalModel(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]string{"ok": "true"})
}

func (s *Server) handleModelStatus(w http.ResponseWriter, r *http.Request) {
	if s.fullBridge == nil {
		writeJSON(w, map[string]interface{}{"running": false})
		return
	}
	writeJSON(w, s.fullBridge.GetLocalModelStatus())
}

func (s *Server) handleEmbeddingStart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost || s.fullBridge == nil {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Path      string `json:"path"`
		GPULayers int    `json:"gpu_layers"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	if err := s.fullBridge.StartEmbeddingModel(req.Path, req.GPULayers); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]string{"ok": "true"})
}

func (s *Server) handleEmbeddingStop(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost || s.fullBridge == nil {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}
	if err := s.fullBridge.StopEmbeddingModel(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]string{"ok": "true"})
}

func (s *Server) handleEmbeddingStatus(w http.ResponseWriter, r *http.Request) {
	if s.fullBridge == nil {
		writeJSON(w, map[string]interface{}{"running": false})
		return
	}
	writeJSON(w, s.fullBridge.GetEmbeddingModelStatus())
}

func (s *Server) handleGPU(w http.ResponseWriter, r *http.Request) {
	if s.fullBridge == nil {
		writeJSON(w, map[string]interface{}{"type": "cpu", "name": "N/A", "vram_mb": 0})
		return
	}
	writeJSON(w, s.fullBridge.DetectGPU())
}

func (s *Server) handleModelSearch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost || s.fullBridge == nil {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Query string `json:"query"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	results, err := s.fullBridge.SearchModels(req.Query)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, results)
}

func (s *Server) handleModelFiles(w http.ResponseWriter, r *http.Request) {
	if s.fullBridge == nil {
		http.Error(w, "not available", http.StatusNotImplemented)
		return
	}
	repoID := r.URL.Query().Get("repo")
	if repoID == "" {
		http.Error(w, "missing repo param", http.StatusBadRequest)
		return
	}
	files := s.fullBridge.GetModelFiles(repoID)
	if files == nil {
		writeJSON(w, []struct{}{})
		return
	}
	writeJSON(w, files)
}

func (s *Server) handleModelDownload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost || s.fullBridge == nil {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		RepoID       string `json:"repo_id"`
		Filename     string `json:"filename"`
		ExpectedSize int64  `json:"expected_size"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	if err := s.fullBridge.DownloadModel(req.RepoID, req.Filename, req.ExpectedSize); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]string{"ok": "true"})
}

func (s *Server) handleDownloadProgress(w http.ResponseWriter, r *http.Request) {
	if s.fullBridge == nil {
		writeJSON(w, []struct{}{})
		return
	}
	p := s.fullBridge.GetDownloadProgress()
	if p == nil {
		writeJSON(w, []struct{}{})
		return
	}
	writeJSON(w, p)
}

func (s *Server) handleDownloadCancel(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost || s.fullBridge == nil {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		RepoID   string `json:"repo_id"`
		Filename string `json:"filename"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	s.fullBridge.CancelDownload(req.RepoID, req.Filename)
	writeJSON(w, map[string]string{"ok": "true"})
}

func (s *Server) handleLlamaCheck(w http.ResponseWriter, r *http.Request) {
	if s.fullBridge == nil {
		writeJSON(w, map[string]bool{"installed": false})
		return
	}
	writeJSON(w, map[string]bool{"installed": s.fullBridge.CheckLlamaInstallation()})
}

func (s *Server) handleLlamaInstall(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost || s.fullBridge == nil {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}
	if err := s.fullBridge.InstallLlamaServer(); err != nil {
		http.Error(w, fmt.Sprintf("install failed: %v", err), http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]string{"ok": "true"})
}

func (s *Server) handleLlamaSkip(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost || s.fullBridge == nil {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}
	if err := s.fullBridge.SkipLlamaGPUInstall(); err != nil {
		http.Error(w, fmt.Sprintf("skip failed: %v", err), http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]string{"ok": "true"})
}

func (s *Server) handleLlamaConfigGet(w http.ResponseWriter, r *http.Request) {
	if s.fullBridge == nil {
		http.Error(w, "not available", http.StatusNotImplemented)
		return
	}
	writeJSON(w, s.fullBridge.GetLlamaConfig())
}

func (s *Server) handleLlamaConfigUpdate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut || s.fullBridge == nil {
		http.Error(w, "PUT only", http.StatusMethodNotAllowed)
		return
	}
	var req config.LlamaConfigUpdate
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	logx.Printf("📥 Backend: Engine mode update received: %s", req.EngineMode)
	if err := s.fullBridge.UpdateLlamaConfig(req); err != nil {
		logx.Printf("❌ Backend: Config update error: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	logx.Printf("✅ Backend: Configuration saved successfully.")
	writeJSON(w, map[string]string{"ok": "true"})
}

// ─── Provider Management ─────────────────────────────────────────

func (s *Server) handleProviders(w http.ResponseWriter, r *http.Request) {
	if s.fullBridge == nil {
		http.Error(w, "not available", http.StatusNotImplemented)
		return
	}
	switch r.Method {
	case http.MethodGet:
		providers := s.fullBridge.GetProviders()
		writeJSON(w, providers)
	case http.MethodPut:
		var req struct {
			Type        provider.ProviderType `json:"type"`
			Name        string                `json:"name"`
			APIKey      string                `json:"api_key"`
			BaseURL     string                `json:"base_url"`
			Model       string                `json:"model"`
			Enabled     bool                  `json:"enabled"`
			Priority    int                   `json:"priority"`
			Temperature float64               `json:"temperature"`
			TopP        float64               `json:"top_p"`
			MaxTokens   int                   `json:"max_tokens"`
			EffortLevel string                `json:"effort_level"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "bad json", http.StatusBadRequest)
			return
		}
		cfg := provider.ProviderConfig{
			Type:        req.Type,
			Name:        req.Name,
			APIKey:      req.APIKey,
			BaseURL:     req.BaseURL,
			Model:       req.Model,
			Enabled:     req.Enabled,
			Priority:    req.Priority,
			Temperature: req.Temperature,
			TopP:        req.TopP,
			MaxTokens:   req.MaxTokens,
			EffortLevel: req.EffortLevel,
		}
		if err := s.fullBridge.UpdateProvider(cfg); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]string{"ok": "true"})
	case http.MethodDelete:
		var req struct {
			Type provider.ProviderType `json:"type"`
			Name string                `json:"name"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "bad json", http.StatusBadRequest)
			return
		}
		if req.Name != "" {
			if err := s.fullBridge.DeleteProvider(req.Type, req.Name); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
		} else {
			if err := s.fullBridge.DeleteProvider(req.Type); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
		}
		writeJSON(w, map[string]string{"ok": "true"})
	default:
		http.Error(w, "GET, PUT, DELETE", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleProviderTest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost || s.fullBridge == nil {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Type        provider.ProviderType `json:"type"`
		Name        string                `json:"name"`
		APIKey      string                `json:"api_key"`
		BaseURL     string                `json:"base_url"`
		Model       string                `json:"model"`
		Temperature float64               `json:"temperature"`
		TopP        float64               `json:"top_p"`
		MaxTokens   int                   `json:"max_tokens"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	cfg := provider.ProviderConfig{
		Type:        req.Type,
		Name:        req.Name,
		APIKey:      req.APIKey,
		BaseURL:     req.BaseURL,
		Model:       req.Model,
		Temperature: req.Temperature,
		TopP:        req.TopP,
		MaxTokens:   req.MaxTokens,
	}
	if err := s.fullBridge.TestProviderConnection(cfg); err != nil {
		writeJSON(w, map[string]interface{}{
			"connected": false,
			"error":     err.Error(),
		})
		return
	}
	writeJSON(w, map[string]interface{}{
		"connected": true,
		"error":     "",
	})
}

// ─── TTS Provider Management (Faz 2) ─────────────────────────────

func (s *Server) handleTTSProviders(w http.ResponseWriter, r *http.Request) {
	if s.fullBridge == nil {
		http.Error(w, "not available", http.StatusNotImplemented)
		return
	}
	switch r.Method {
	case http.MethodGet:
		providers := s.fullBridge.GetTTSProviders()
		writeJSON(w, providers)
	case http.MethodPut:
		var req struct {
			Type     tts.ProviderType `json:"type"`
			Name     string           `json:"name"`
			APIKey   string           `json:"api_key"`
			Voice    string           `json:"voice"`
			Enabled  bool             `json:"enabled"`
			Priority int              `json:"priority"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "bad json", http.StatusBadRequest)
			return
		}
		cfg := tts.ProviderConfig{
			Type:     req.Type,
			Name:     req.Name,
			APIKey:   req.APIKey,
			Voice:    req.Voice,
			Enabled:  req.Enabled,
			Priority: req.Priority,
		}
		if err := s.fullBridge.UpdateTTSProvider(cfg); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]string{"ok": "true"})
	case http.MethodDelete:
		var req struct {
			Type tts.ProviderType `json:"type"`
			Name string           `json:"name"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "bad json", http.StatusBadRequest)
			return
		}
		if req.Name != "" {
			if err := s.fullBridge.DeleteTTSProvider(req.Type, req.Name); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
		} else {
			if err := s.fullBridge.DeleteTTSProvider(req.Type); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
		}
		writeJSON(w, map[string]string{"ok": "true"})
	default:
		http.Error(w, "GET, PUT, DELETE", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleTTSProviderTest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost || s.fullBridge == nil {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Type   tts.ProviderType `json:"type"`
		Name   string           `json:"name"`
		APIKey string           `json:"api_key"`
		Voice  string           `json:"voice"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	cfg := tts.ProviderConfig{
		Type:   req.Type,
		Name:   req.Name,
		APIKey: req.APIKey,
		Voice:  req.Voice,
	}
	if err := s.fullBridge.TestTTSProviderConnection(cfg); err != nil {
		writeJSON(w, map[string]interface{}{
			"connected": false,
			"error":     err.Error(),
		})
		return
	}
	writeJSON(w, map[string]interface{}{
		"connected": true,
		"error":     "",
	})
}

// ─── TTS Voice Store (Faz 2.6 — local, offline Piper voices) ─────

func (s *Server) handleTTSVoices(w http.ResponseWriter, r *http.Request) {
	if s.fullBridge == nil {
		http.Error(w, "not available", http.StatusNotImplemented)
		return
	}
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, map[string]interface{}{
			"catalog":       s.fullBridge.GetTTSVoiceCatalog(),
			"local":         s.fullBridge.GetLocalTTSVoices(),
			"downloads":     s.fullBridge.GetTTSVoiceDownloadProgress(),
			"selected_path": s.fullBridge.GetSelectedTTSVoicePath(),
		})
	case http.MethodDelete:
		var req struct {
			ID string `json:"id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "bad json", http.StatusBadRequest)
			return
		}
		if err := s.fullBridge.DeleteTTSVoice(req.ID); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]string{"ok": "true"})
	default:
		http.Error(w, "GET, DELETE", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleTTSVoiceDownload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost || s.fullBridge == nil {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Locale  string `json:"locale"`
		Name    string `json:"name"`
		Quality string `json:"quality"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	if err := s.fullBridge.DownloadTTSVoice(req.Locale, req.Name, req.Quality); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]string{"ok": "true"})
}

func (s *Server) handleTTSVoiceSelect(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost || s.fullBridge == nil {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	if err := s.fullBridge.SelectTTSVoice(req.ID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]string{"ok": "true"})
}

// handleProviderModels lists the models available for a provider type by
// calling its ListModels directly — a generic counterpart to the
// OpenRouter-specific /api/openrouter/models, for providers (OpenCode Zen,
// OpenCode Go, and any future OpenAI-compatible one) that expose a plain
// GET /models endpoint but no bespoke pricing/metadata endpoint to browse.
func (s *Server) handleProviderModels(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Type    provider.ProviderType `json:"type"`
		APIKey  string                `json:"api_key"`
		BaseURL string                `json:"base_url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}

	p, err := provider.NewProvider(provider.ProviderConfig{
		Type:    req.Type,
		APIKey:  req.APIKey,
		BaseURL: req.BaseURL,
	})
	if err != nil {
		writeJSON(w, map[string]interface{}{"status": "error", "error": err.Error()})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	models, err := p.ListModels(ctx)
	if err != nil {
		writeJSON(w, map[string]interface{}{"status": "error", "error": err.Error()})
		return
	}
	writeJSON(w, map[string]interface{}{"status": "ok", "models": models})
}

func (s *Server) handleActiveProvider(w http.ResponseWriter, r *http.Request) {
	if s.fullBridge == nil {
		http.Error(w, "bridge not available", http.StatusServiceUnavailable)
		return
	}
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, map[string]string{"provider": s.fullBridge.GetActiveProvider()})
	case http.MethodPut:
		var req struct {
			Provider string `json:"provider"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "bad json", http.StatusBadRequest)
			return
		}
		s.fullBridge.SetActiveProvider(req.Provider)
		writeJSON(w, map[string]string{"ok": "true"})
	default:
		http.Error(w, "GET or PUT", http.StatusMethodNotAllowed)
	}
}

// ─── Orchestra Handlers ──────────────────────────────────────────────

func (s *Server) handleOrchestraConfig(w http.ResponseWriter, r *http.Request) {
	if s.fullBridge == nil {
		http.Error(w, "bridge not available", http.StatusServiceUnavailable)
		return
	}
	switch r.Method {
	case http.MethodGet:
		cfg := s.fullBridge.GetOrchestraConfig()
		writeJSON(w, cfg)
	case http.MethodPut:
		var cfg orchestra.OrchestraConfig
		if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
			http.Error(w, "bad json", http.StatusBadRequest)
			return
		}
		if err := s.fullBridge.UpdateOrchestraConfig(cfg); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]string{"ok": "true"})
	default:
		http.Error(w, "GET or PUT", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleAgentChat(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost || s.fullBridge == nil {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		ProjectPath string `json:"project_path"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	if req.ProjectPath == "" {
		http.Error(w, "project_path required", http.StatusBadRequest)
		return
	}
	id := s.fullBridge.NewAgentChat(req.ProjectPath)
	writeJSON(w, map[string]string{"id": id})
}

// ─── Agent Handlers ──────────────────────────────────────────────────

func (s *Server) handleAgentEnabled(w http.ResponseWriter, r *http.Request) {
	if s.fullBridge == nil {
		http.Error(w, "bridge not available", http.StatusServiceUnavailable)
		return
	}

	switch r.Method {
	case http.MethodGet:
		writeJSON(w, map[string]bool{"enabled": s.fullBridge.GetAgentEnabled()})
	case http.MethodPut:
		var req struct {
			Enabled bool `json:"enabled"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "bad json", http.StatusBadRequest)
			return
		}
		if err := s.fullBridge.SetAgentEnabled(req.Enabled); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]bool{"ok": true})
	default:
		http.Error(w, "GET or PUT", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleAgentPermission(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost || s.fullBridge == nil {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		RequestID string `json:"request_id"`
		Policy    string `json:"policy"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	if err := s.fullBridge.HandleAgentPermission(req.RequestID, req.Policy); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(w, map[string]bool{"ok": true})
}

func (s *Server) handleAgentPermissions(w http.ResponseWriter, r *http.Request) {
	if s.fullBridge == nil {
		http.Error(w, "bridge not available", http.StatusServiceUnavailable)
		return
	}
	switch r.Method {
	case http.MethodGet:
		perms := s.fullBridge.GetAgentPermissions()
		writeJSON(w, perms)
	case http.MethodDelete:
		id := r.URL.Query().Get("id")
		if id != "" {
			if err := s.fullBridge.RevokeAgentPermission(id); err != nil {
				http.Error(w, err.Error(), http.StatusNotFound)
				return
			}
		} else {
			s.fullBridge.ClearAgentPermissions()
		}
		writeJSON(w, map[string]bool{"ok": true})
	default:
		http.Error(w, "GET or DELETE", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleAgentAutoPermission(w http.ResponseWriter, r *http.Request) {
	if s.fullBridge == nil {
		http.Error(w, "bridge not available", http.StatusServiceUnavailable)
		return
	}

	switch r.Method {
	case http.MethodGet:
		writeJSON(w, map[string]bool{"enabled": s.fullBridge.GetAgentAutoPermission()})
	case http.MethodPut:
		var req struct {
			Enabled bool `json:"enabled"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "bad json", http.StatusBadRequest)
			return
		}
		if err := s.fullBridge.SetAgentAutoPermission(req.Enabled); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]bool{"ok": true})
	default:
		http.Error(w, "GET or PUT", http.StatusMethodNotAllowed)
	}
}

// ─── WhatsApp Handlers ────────────────────────────────────────────

func (s *Server) handleWhatsAppStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet || s.fullBridge == nil {
		http.Error(w, "GET only", http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, s.fullBridge.WhatsAppStatus())
}

func (s *Server) handleWhatsAppStart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost || s.fullBridge == nil {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}
	if err := s.fullBridge.StartWhatsApp(r.Context()); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, s.fullBridge.WhatsAppStatus())
}

func (s *Server) handleWhatsAppStop(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost || s.fullBridge == nil {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}
	s.fullBridge.StopWhatsApp()
	writeJSON(w, map[string]bool{"ok": true})
}

func (s *Server) handleWhatsAppLogout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost || s.fullBridge == nil {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}
	if err := s.fullBridge.LogoutWhatsApp(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]bool{"ok": true})
}

func (s *Server) handleWhatsAppSend(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost || s.fullBridge == nil {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		JID  string `json:"jid"`
		Text string `json:"text"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	msgID, err := s.fullBridge.WhatsAppSend(r.Context(), req.JID, req.Text)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]string{"id": msgID})
}

func (s *Server) handleWhatsAppSearch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet || s.fullBridge == nil {
		http.Error(w, "GET only", http.StatusMethodNotAllowed)
		return
	}
	query := r.URL.Query().Get("q")
	msgs, err := s.fullBridge.WhatsAppSearch(query, 50)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, msgs)
}

func (s *Server) handleWhatsAppChats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet || s.fullBridge == nil {
		http.Error(w, "GET only", http.StatusMethodNotAllowed)
		return
	}
	chats, err := s.fullBridge.WhatsAppGetChats()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, chats)
}

func (s *Server) handleWhatsAppMessages(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet || s.fullBridge == nil {
		http.Error(w, "GET only", http.StatusMethodNotAllowed)
		return
	}
	chatJID := r.URL.Query().Get("jid")
	if chatJID == "" {
		http.Error(w, "jid param required", http.StatusBadRequest)
		return
	}
	msgs, err := s.fullBridge.WhatsAppGetMessages(chatJID, 50)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, msgs)
}

func (s *Server) handleWhatsAppAvatar(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet || s.fullBridge == nil {
		http.Error(w, "GET only", http.StatusMethodNotAllowed)
		return
	}
	jid := r.URL.Query().Get("jid")
	if jid == "" {
		http.Error(w, "jid param required", http.StatusBadRequest)
		return
	}
	full := r.URL.Query().Get("full") == "1"
	data, err := s.fullBridge.WhatsAppAvatar(jid, full)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if len(data) == 0 {
		http.Error(w, "no avatar", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "image/jpeg")
	w.Header().Set("Cache-Control", "public, max-age=86400")
	_, _ = w.Write(data)
}

func (s *Server) handleWebSearchSettings(w http.ResponseWriter, r *http.Request) {
	if s.fullBridge == nil {
		http.Error(w, "not available", http.StatusServiceUnavailable)
		return
	}
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, map[string]bool{"enabled": s.fullBridge.GetWebSearchEnabled()})
	case http.MethodPost:
		var req struct {
			Enabled bool `json:"enabled"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "bad json", http.StatusBadRequest)
			return
		}
		if err := s.fullBridge.UpdateWebSearchConfig(req.Enabled); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]bool{"enabled": req.Enabled})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleWhatsAppStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet || s.fullBridge == nil {
		http.Error(w, "GET only", http.StatusMethodNotAllowed)
		return
	}
	total, last24h, err := s.fullBridge.WhatsAppStats()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]interface{}{
		"total":    total,
		"last_24h": last24h,
	})
}

func (s *Server) handleWhatsAppChatMode(w http.ResponseWriter, r *http.Request) {
	if s.fullBridge == nil {
		http.Error(w, "not available", http.StatusMethodNotAllowed)
		return
	}
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, map[string]bool{"enabled": s.fullBridge.GetWhatsAppChatMode()})
	case http.MethodPost:
		var req struct {
			Enabled bool `json:"enabled"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "bad json", http.StatusBadRequest)
			return
		}
		s.fullBridge.SetWhatsAppChatMode(req.Enabled)
		writeJSON(w, map[string]bool{"enabled": req.Enabled})
	default:
		http.Error(w, "GET or POST", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleWhatsAppChatStream(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost || s.fullBridge == nil {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Message string `json:"message"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	ctx := r.Context()
	streamCh := s.fullBridge.WhatsAppChatStream(ctx, req.Message)
	streamSSE(ctx, w, flusher, streamCh)
}

// ─── Skill Handlers ──────────────────────────────────────────────

func (s *Server) handleListSkills(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "GET only", http.StatusMethodNotAllowed)
		return
	}
	if s.fullBridge == nil {
		http.Error(w, "bridge not available", http.StatusServiceUnavailable)
		return
	}
	writeJSON(w, s.fullBridge.ListSkills())
}

func (s *Server) handleInstallSkill(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}
	if s.fullBridge == nil {
		http.Error(w, "bridge not available", http.StatusServiceUnavailable)
		return
	}
	var req struct {
		Path string `json:"path"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	def, err := s.fullBridge.InstallSkill(req.Path)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, def)
}

func (s *Server) handleRemoveSkill(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "DELETE only", http.StatusMethodNotAllowed)
		return
	}
	if s.fullBridge == nil {
		http.Error(w, "bridge not available", http.StatusServiceUnavailable)
		return
	}
	name := r.PathValue("name")
	if err := s.fullBridge.RemoveSkill(name); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, map[string]bool{"ok": true})
}

func (s *Server) handleGetSkill(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "GET only", http.StatusMethodNotAllowed)
		return
	}
	if s.fullBridge == nil {
		http.Error(w, "bridge not available", http.StatusServiceUnavailable)
		return
	}
	name := r.PathValue("name")
	def, err := s.fullBridge.GetSkill(name)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, def)
}

func (s *Server) handleSetActiveSkills(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		http.Error(w, "PUT only", http.StatusMethodNotAllowed)
		return
	}
	if s.fullBridge == nil {
		http.Error(w, "bridge not available", http.StatusServiceUnavailable)
		return
	}
	var req struct {
		Names []string `json:"names"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	if err := s.fullBridge.SetActiveSkills(req.Names); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, map[string]bool{"ok": true})
}

func (s *Server) handleGetActiveSkills(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "GET only", http.StatusMethodNotAllowed)
		return
	}
	if s.fullBridge == nil {
		http.Error(w, "bridge not available", http.StatusServiceUnavailable)
		return
	}
	writeJSON(w, map[string][]string{"names": s.fullBridge.GetActiveSkills()})
}

func (s *Server) handleMemoryExplicitSave(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost || s.fullBridge == nil {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}
	var body struct {
		Content string `json:"content"`
		Tags    string `json:"tags"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Content == "" {
		http.Error(w, "content required", http.StatusBadRequest)
		return
	}
	if err := s.fullBridge.SaveExplicitMemory(body.Content, body.Tags); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]bool{"ok": true})
}

func (s *Server) handleMemoryExplicitDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost || s.fullBridge == nil {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}
	var body struct {
		Pattern string `json:"pattern"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Pattern == "" {
		http.Error(w, "pattern required", http.StatusBadRequest)
		return
	}
	deleted, err := s.fullBridge.DeleteExplicitMemory(body.Pattern)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]any{"ok": true, "deleted": deleted})
}

func (s *Server) handleMemoryImportText(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost || s.fullBridge == nil {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}
	var body struct {
		Content string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || strings.TrimSpace(body.Content) == "" {
		http.Error(w, "content required", http.StatusBadRequest)
		return
	}
	factsSaved, styleUpdated, err := s.fullBridge.ImportMemoryFromText(r.Context(), body.Content)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]any{"ok": true, "facts_saved": factsSaved, "style_updated": styleUpdated})
}

// handleTTSSynthesize is handleTranscribe's reverse direction (text in,
// audio out instead of audio in, text out) — see internal/tts's package
// doc and docs/plans/PLAN_voice_live_mode_faz1.md's 1.3. Responds with raw
// WAV bytes rather than base64-in-JSON, mirroring how handleTranscribe
// already reads its audio input as a raw body rather than JSON+base64.
func (s *Server) handleTTSSynthesize(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost || s.fullBridge == nil {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}
	var body struct {
		Text string `json:"text"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || strings.TrimSpace(body.Text) == "" {
		http.Error(w, "text required", http.StatusBadRequest)
		return
	}
	audio, err := s.fullBridge.SynthesizeSpeech(body.Text)
	if err != nil {
		logx.Error("Synthesize error", "err", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "audio/wav")
	w.Write(audio)
}

// handleTTSFiller returns one cached, local-only "thinking" filler sound
// (Faz 3) — GET, not POST, since it takes no input.
func (s *Server) handleTTSFiller(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet || s.fullBridge == nil {
		http.Error(w, "GET only", http.StatusMethodNotAllowed)
		return
	}
	audio, err := s.fullBridge.GetTTSFillerSound()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "audio/wav")
	w.Write(audio)
}

func (s *Server) handleMemoryInsight(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost || s.fullBridge == nil {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}
	var body struct {
		WindowDays int    `json:"window_days"`
		Lang       string `json:"lang"`
	}
	// Body is optional — a plain POST with no body just uses the default
	// window (see GenerateSelfInsight's windowDays<=0 handling) and Turkish.
	_ = json.NewDecoder(r.Body).Decode(&body)

	insight, err := s.fullBridge.GenerateSelfInsight(r.Context(), body.WindowDays, body.Lang)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]any{"ok": true, "insight": insight})
}

func (s *Server) handleMemoryExport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet || s.fullBridge == nil {
		http.Error(w, "GET only", http.StatusMethodNotAllowed)
		return
	}
	data, err := s.fullBridge.ExportMemories()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", "attachment; filename=memories.json")
	_, _ = w.Write(data)
}

func (s *Server) handleMemoryImport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost || s.fullBridge == nil {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}
	data, err := io.ReadAll(io.LimitReader(r.Body, 50*1024*1024))
	if err != nil {
		http.Error(w, "read error", http.StatusBadRequest)
		return
	}
	imported, err := s.fullBridge.ImportMemories(data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]any{"ok": true, "imported": imported})
}

func (s *Server) handleMemoryStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet || s.fullBridge == nil {
		http.Error(w, "GET only", http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, s.fullBridge.GetMemoryStats())
}

// handleUsageStats serves the Settings usage-stats tab. ?days=N sets the
// lookback window (default 30).
func (s *Server) handleUsageStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet || s.fullBridge == nil {
		http.Error(w, "GET only", http.StatusMethodNotAllowed)
		return
	}
	days := 30
	if raw := r.URL.Query().Get("days"); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			days = parsed
		}
	}
	writeJSON(w, s.fullBridge.GetUsageStats(days))
}

func (s *Server) handleMemoryFilteredSearch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet || s.fullBridge == nil {
		http.Error(w, "GET only", http.StatusMethodNotAllowed)
		return
	}
	q := r.URL.Query().Get("q")
	if q == "" {
		http.Error(w, "q parameter required", http.StatusBadRequest)
		return
	}
	topK := 10
	since := r.URL.Query().Get("since")
	if since != "" {
		if _, err := time.Parse(time.RFC3339, since); err != nil {
			http.Error(w, "invalid since format: use RFC3339 (e.g. 2026-06-01T00:00:00Z)", http.StatusBadRequest)
			return
		}
	}
	tag := r.URL.Query().Get("tag")
	results := s.fullBridge.FilteredMemorySearch(q, topK, since, tag)
	if results == nil {
		writeJSON(w, []struct{}{})
		return
	}
	writeJSON(w, results)
}

// handleShutdown accepts POST and initiates graceful shutdown. The response
// is flushed before signaling so the HTTP server can complete this request
// before it stops.
//
// This only ever requests a shutdown via internal/shutdown — it used to
// *also* call s.fullBridge.Shutdown() directly first, but that ran the
// entire teardown (App.Shutdown, guarded by sync.Once) synchronously from
// inside this very HTTP handler, while the request itself was still in
// flight from the server's perspective. webserver.Stop() closes its
// listener via an async srv.Shutdown() that waits for in-flight requests to
// finish — including this one — so the two shutdown paths (the direct call
// here, and the one main()'s deferred a.Shutdown(ctx) runs) were racing
// each other with the store/WhatsApp/etc. teardown potentially running
// while the server could still be accepting other requests. sync.Once made
// a second *call* to Shutdown harmless, but didn't fix that overlap. A
// single, main()-driven shutdown path removes it entirely.
//
// Used to self-signal SIGINT via os.Process.Signal(os.Interrupt) directly —
// a silent no-op on Windows (Go's os package only implements Process.Signal
// for os.Kill there), so a Windows-hosted backend never actually stopped on
// POST /api/shutdown. shutdown.Request() has no such gap: main() selects on
// it alongside its OS signal channel on every platform.
func (s *Server) handleShutdown(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, map[string]string{"status": "shutting_down"})

	// Ask main() to run the deferred Shutdown chain (WAL checkpoint, DB
	// flush, in-flight HTTP drain). os.Exit() would skip deferred functions
	// and corrupt SQLite WAL files.
	shutdown.Request()
}

// ─── Task Lists ──────────────────────────────────────────────────

func (s *Server) handleTaskLists(w http.ResponseWriter, r *http.Request) {
	if s.fullBridge == nil {
		http.Error(w, "not available", http.StatusMethodNotAllowed)
		return
	}

	switch r.Method {
	case http.MethodGet:
		writeJSON(w, s.fullBridge.ListTaskLists())
	case http.MethodPost:
		var req struct {
			ChatID string   `json:"chat_id"`
			Title  string   `json:"title"`
			Items  []string `json:"items"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "bad json", http.StatusBadRequest)
			return
		}
		if len(req.Items) == 0 {
			http.Error(w, "items required", http.StatusBadRequest)
			return
		}
		tl, err := s.fullBridge.CreateTaskList(req.ChatID, req.Title, req.Items)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, tl)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleTaskListByID(w http.ResponseWriter, r *http.Request) {
	if s.fullBridge == nil {
		http.Error(w, "not available", http.StatusMethodNotAllowed)
		return
	}

	id := strings.TrimPrefix(r.URL.Path, "/api/tasklists/")
	if id == "" {
		http.Error(w, "missing id", http.StatusBadRequest)
		return
	}

	parts := strings.SplitN(id, "/", 2)
	listID := parts[0]
	subAction := ""
	if len(parts) > 1 {
		subAction = parts[1]
	}

	switch {
	case subAction == "start" && r.Method == http.MethodPost:
		// Engine.Start spawns a goroutine that outlives this handler (it
		// returns immediately after `go e.run(...)`), so r.Context() is the
		// wrong parent - net/http cancels it the instant this handler
		// returns, which raced the run loop's very first ctx.Done() check
		// and made every list pause after zero items, every time (reported
		// live: Start always immediately re-paused with 0 items processed).
		// The engine's own Stop()/active-map bookkeeping already owns the
		// list's lifecycle, so a background context here is correct, not
		// just a workaround.
		if err := s.fullBridge.StartTaskList(context.Background(), listID); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]string{"status": "started"})
	case subAction == "stop" && r.Method == http.MethodPost:
		s.fullBridge.StopTaskList(listID)
		writeJSON(w, map[string]string{"status": "stopped"})
	case subAction == "" && r.Method == http.MethodGet:
		tl, err := s.fullBridge.GetTaskList(listID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		writeJSON(w, tl)
	case subAction == "" && r.Method == http.MethodDelete:
		if err := s.fullBridge.DeleteTaskList(listID); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]string{"status": "deleted"})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}
