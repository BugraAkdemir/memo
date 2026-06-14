package webserver

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"memo/internal/api"
	"memo/internal/config"
	"memo/internal/orchestra"
	"memo/internal/provider"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

// ─── Chat Streaming (SSE) ───────────────────────────────────────

func (s *Server) handleSendStream(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost || s.fullBridge == nil {
		http.Error(w, "not available", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Message string `json:"message"`
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
	ch := s.fullBridge.SendMessageStream(ctx, req.Message)

	for {
		select {
		case <-ctx.Done():
			return
		case chunk, ok := <-ch:
			if !ok {
				return
			}
			data, err := json.Marshal(chunk)
			if err != nil {
				continue
			}
			fmt.Fprintf(w, "data: %s\n\n", string(data))
			flusher.Flush()

			if chunk.Done {
				return
			}
		}
	}
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

	mimeType := header.Header.Get("Content-Type")
	isImage := false
	if mimeType != "" {
		if strings.HasPrefix(mimeType, "image") {
			isImage = true
		}
	} else {
		ext := strings.ToLower(filepath.Ext(tmpFilePath))
		if ext == ".png" || ext == ".jpg" || ext == ".jpeg" || ext == ".gif" || ext == ".webp" || ext == ".bmp" {
			isImage = true
		}
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
	var ch <-chan api.StreamChunk
	if isImage {
		ch = s.fullBridge.SendMessageWithImageStream(ctx, msg, tmpFilePath)
	} else {
		ch = s.fullBridge.SendMessageWithFileStream(ctx, msg, tmpFilePath)
	}

	for {
		select {
		case <-ctx.Done():
			return
		case chunk, ok := <-ch:
			if !ok {
				return
			}
			data, err := json.Marshal(chunk)
			if err != nil {
				continue
			}
			fmt.Fprintf(w, "data: %s\n\n", string(data))
			flusher.Flush()

			if chunk.Done {
				return
			}
		}
	}
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

func (s *Server) handleRemoteAccess(w http.ResponseWriter, r *http.Request) {
	if s.fullBridge == nil {
		http.Error(w, "not available", http.StatusNotImplemented)
		return
	}
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, s.fullBridge.GetRemoteAccessStatus())
	case http.MethodPut:
		var req struct {
			Enabled        *bool  `json:"enabled"`
			Port           int    `json:"port"`
			NgrokMode      bool   `json:"ngrok_mode"`
			NgrokToken     string `json:"ngrok_token"`
			NgrokAutoStart *bool  `json:"ngrok_auto_start"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "bad json", http.StatusBadRequest)
			return
		}
		if req.NgrokAutoStart != nil {
			s.fullBridge.SetNgrokAutoStart(*req.NgrokAutoStart)
		}
		if req.Enabled != nil {
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
		writeJSON(w, map[string]string{"ok": "true"})
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
		RepoID   string `json:"repo_id"`
		Filename string `json:"filename"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	if err := s.fullBridge.DownloadModel(req.RepoID, req.Filename); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]string{"ok": "true"})
}

func (s *Server) handleDownloadProgress(w http.ResponseWriter, r *http.Request) {
	if s.fullBridge == nil {
		writeJSON(w, map[string]interface{}{"active": false})
		return
	}
	p := s.fullBridge.GetDownloadProgress()
	if p == nil {
		writeJSON(w, map[string]interface{}{"active": false})
		return
	}
	writeJSON(w, p)
}

func (s *Server) handleDownloadCancel(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost || s.fullBridge == nil {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}
	s.fullBridge.CancelDownload()
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
	var req config.LlamaConfig
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	log.Printf("📥 Backend: Engine mode update received: %s", req.EngineMode)
	if err := s.fullBridge.UpdateLlamaConfig(req); err != nil {
		log.Printf("❌ Backend: Config update error: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	log.Printf("✅ Backend: Configuration saved successfully.")
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
			Provider provider.ProviderType `json:"provider"`
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

	for {
		select {
		case <-ctx.Done():
			return
		case chunk, ok := <-streamCh:
			if !ok {
				return
			}
			if chunk.FinishReason == "agent_event" {
				fmt.Fprintf(w, "data: %s\n\n", chunk.Content)
			} else if chunk.Content != "" {
				fmt.Fprintf(w, "data: %s\n\n", chunk.Content)
			}
			if chunk.Done {
				fmt.Fprintf(w, "data: [DONE]\n\n")
				return
			}
			flusher.Flush()
		}
	}
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
		writeJSON(w, map[string]string{"error": err.Error()})
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
		writeJSON(w, map[string]string{"error": err.Error()})
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
		writeJSON(w, map[string]string{"error": err.Error()})
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
		writeJSON(w, map[string]string{"error": err.Error()})
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
