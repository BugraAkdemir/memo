package webserver

import (
	"encoding/json"
	"fmt"
	"net/http"
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
	// For now, fall back to sync send and return full reply
	reply := s.bridge.SendMessage(req.Message)
	writeJSON(w, map[string]string{"reply": reply})
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
	writeJSON(w, map[string]string{"version": s.fullBridge.GetVersion()})
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
	b64 := s.fullBridge.GetImageBase64(path)
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

// ─── Recording ──────────────────────────────────────────────────

func (s *Server) handleRecordingStart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost || s.fullBridge == nil {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}
	if err := s.fullBridge.StartRecording(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]string{"ok": "true"})
}

func (s *Server) handleRecordingStop(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost || s.fullBridge == nil {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}
	text, err := s.fullBridge.StopRecordingAndTranscribe()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]string{"text": text})
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
			Enabled bool `json:"enabled"`
			Port    int  `json:"port"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "bad json", http.StatusBadRequest)
			return
		}
		if err := s.fullBridge.SetRemoteAccess(req.Enabled, req.Port); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
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
