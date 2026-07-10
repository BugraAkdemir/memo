package app

import (
	"context"
	"fmt"
	"memo/internal/logx"
	"strings"
	"time"

	"memo/internal/api"
	"memo/internal/config"
	"memo/internal/memory"
	"memo/internal/models"
	"memo/internal/provider"
)

// isEmbeddingBackendDown reports whether err means the embedding endpoint is
// simply unreachable (no local embedding server running). This is expected when
// the user runs an API/Orchestra-only setup, so we log it but don't surface a
// scary "memory error" toast on every single message.
func isEmbeddingBackendDown(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "connection refused") ||
		strings.Contains(msg, "connect: connection refused") ||
		strings.Contains(msg, "no such host") ||
		strings.Contains(msg, "context deadline exceeded") ||
		strings.Contains(msg, "EOF")
}

// isLLMErrorReply reports whether reply is one of callLLM's synthetic
// "⚠️ ..." error strings (model not loaded, provider error, empty response,
// unreadable attachment, etc.) rather than a genuine model response. Every
// error path in llm.go and chat.go consistently uses this prefix — the same
// convention the streaming path's recordStreamError relies on — so it's a
// reliable signal that reply must not be indexed into RAG memory.
func isLLMErrorReply(reply string) bool {
	return strings.HasPrefix(reply, "⚠️")
}

func (a *App) saveMemoryAsync(userMsg, reply string) {
	if reply == "" || isLLMErrorReply(reply) || !a.GetMemoryEnabled() {
		return
	}
	select {
	case a.memorySaveCh <- saveTask{userMsg: userMsg, reply: reply}:
	case <-time.After(2 * time.Second):
		logx.Info("WARN: memory save channel full, dropping")
		a.emitEvent("memory:error", "Hafıza kaydetme kuyruğu dolu; bu mesaj hatırlanmayabilir")
	}
}

func (a *App) memorySaveWorker() {
	for task := range a.memorySaveCh {
		a.saveMemorySync(a.lifecycleCtx, task.userMsg, task.reply)
	}
}

func (a *App) saveMemorySync(ctx context.Context, userMsg, reply string) {
	if !a.GetMemoryEnabled() {
		return
	}
	start := time.Now()

	// Hold the lock only long enough to grab the store reference so that the
	// heavy embedding I/O below does not block concurrent memory reads/writes.
	a.storeMu.RLock()
	store := a.store
	a.storeMu.RUnlock()
	if store == nil {
		logx.Info("MEMORY SAVE SKIPPED: store not initialized")
		a.emitEvent("memory:error", "Hafıza kaydedilemedi: depo başlatılmamış")
		return
	}

	mctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	if err := store.SaveInteraction(mctx, userMsg, reply); err != nil {
		logx.Printf("LATENCY app.memory_save_sync total_ms=%d status=error", time.Since(start).Milliseconds())
		logx.Printf("MEMORY SAVE FAILED: %v", err)
		if !isEmbeddingBackendDown(err) {
			a.emitEvent("memory:error", fmt.Sprintf("Hafıza kaydedilemedi: %v", err))
		}
	} else {
		logx.Printf("LATENCY app.memory_save_sync total_ms=%d status=ok", time.Since(start).Milliseconds())
		logx.Printf("Memory saved: %q → %d chars reply", truncateLog(userMsg, 60), len(reply))
		// Only signal here, once the async save has actually completed — lets
		// clients (e.g. the terminal REPL) show a "memory saved" confirmation
		// that's tied to a real write instead of just assuming it happened.
		a.emitEvent("memory:saved", "")
		a.syncMu.RLock()
		sm := a.syncManager
		a.syncMu.RUnlock()
		if sm != nil {
			sm.Increment()
		}
	}
}

func (a *App) retrieveMemory(ctx context.Context, query string) []memory.MemoryResult {
	a.storeMu.RLock()
	defer a.storeMu.RUnlock()
	if a.store == nil {
		logx.Info("Memory: store not initialized, skipping retrieve")
		return nil
	}
	start := time.Now()
	rctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	m, err := a.store.RetrieveContext(rctx, query, a.cfg.Memory.TopK, a.cfg.Memory.MinSimilarity)
	if err != nil {
		logx.Printf("LATENCY app.retrieve_memory total_ms=%d status=error", time.Since(start).Milliseconds())
		logx.Printf("MEMORY RETRIEVE FAILED: %v", err)
		// Embedding backend unreachable is expected in API/Orchestra-only mode —
		// degrade silently instead of flooding the UI with error toasts.
		if !isEmbeddingBackendDown(err) {
			a.emitEvent("memory:error", fmt.Sprintf("Hafıza okunamadı: %v", err))
		}
		return nil
	}
	logx.Printf("LATENCY app.retrieve_memory total_ms=%d returned=%d", time.Since(start).Milliseconds(), len(m))
	if len(m) > 0 {
		logx.Printf("Memory: found %d relevant memories (best=%.0f%%)", len(m), m[0].Similarity*100)
	}
	return m
}

func (a *App) reinitMemoryStore(client *api.Client, model string) {
	// Remember the old store, then set a.store = nil so read-side guards
	// (retrieveMemory etc.) return early while we build the replacement.
	// If NewStore fails we restore the old store; on success we close the
	// old one under lock and swap in the new one — no permanent nil window.
	a.storeMu.Lock()
	oldStore := a.store
	a.store = nil
	a.storeMu.Unlock()

	embeddingFunc := memory.NewEmbeddingFunc(client, model)
	newStore, err := memory.NewStore(memory.StoreConfig{
		Dir:           a.cfg.Memory.PersistDir,
		Dimension:     a.cfg.Memory.EmbeddingDimension,
		EmbeddingFunc: embeddingFunc,
	})
	if err != nil {
		logx.Printf("WARN: memory re-init: %v (restoring old store)", err)
		a.emitEvent("memory_store_error", err.Error())
		a.storeMu.Lock()
		a.store = oldStore
		a.storeMu.Unlock()
		return
	}

	newStore.SetConsolidationFunc(a.mergeMemoriesLLM)

	a.storeMu.Lock()
	if oldStore != nil {
		if err := oldStore.Close(); err != nil {
			logx.Printf("WARN: memory store close during re-init: %v", err)
		}
	}
	a.store = newStore
	a.storeMu.Unlock()
	logx.Info("Memory store re-initialized")
}

// DebugMemorySearch searches memory WITHOUT similarity filter — for debugging.
func (a *App) DebugMemorySearch(query string) []memory.MemoryResult {
	a.storeMu.RLock()
	defer a.storeMu.RUnlock()
	if a.store == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return a.store.DebugSearch(ctx, query, 10)
}

// GetMemoryCount returns the number of stored memory entries.
func (a *App) GetMemoryCount() int {
	a.storeMu.RLock()
	defer a.storeMu.RUnlock()
	if a.store == nil {
		return 0
	}
	return a.store.Count()
}

// ClearAllMemory removes all stored memory entries.
func (a *App) ClearAllMemory() error {
	a.storeMu.Lock()
	defer a.storeMu.Unlock()
	if a.store == nil {
		return fmt.Errorf("no memory store")
	}
	logx.Info("Clearing all memory...")
	return a.store.ClearAll()
}

// ListMemoryFiles lists the on-disk gob files in the memory store.
func (a *App) ListMemoryFiles() []memory.GobFileInfo {
	a.storeMu.RLock()
	defer a.storeMu.RUnlock()
	if a.store == nil {
		return nil
	}
	return a.store.ListGobFiles()
}

// DeleteMemoryFile deletes a specific gob file from the memory store.
func (a *App) DeleteMemoryFile(relPath string) error {
	a.storeMu.Lock()
	defer a.storeMu.Unlock()
	if a.store == nil {
		return fmt.Errorf("no memory store")
	}
	logx.Printf("Deleting memory file: %s", relPath)
	return a.store.DeleteGobFile(relPath)
}

// GetMemorySettings returns memory configuration.
func (a *App) GetMemorySettings() config.MemoryConfig {
	return a.cfg.Memory
}

// UpdateMemorySettings updates topK and minSimilarity in the memory config.
func (a *App) UpdateMemorySettings(topK int, minSimilarity float32) error {
	if topK < 1 || topK > 50 {
		return fmt.Errorf("top_k must be between 1 and 50")
	}
	if minSimilarity <= 0 || minSimilarity > 1 {
		return fmt.Errorf("min_similarity must be between 0.01 and 1")
	}

	a.cfg.Memory.TopK = topK
	a.cfg.Memory.MinSimilarity = minSimilarity
	if err := config.Save(a.cfg); err != nil {
		return err
	}
	logx.Printf("Memory settings updated: top_k=%d min_similarity=%.2f", topK, minSimilarity)
	return nil
}

// GetMemoryEnabled reports whether memory is enabled.
func (a *App) GetMemoryEnabled() bool {
	a.cfgMu.RLock()
	defer a.cfgMu.RUnlock()
	return a.cfg.Memory.MemoryEnabled
}

// mergeMemoriesLLM sends two memory contents to the active provider and returns
// a single merged memory. Used as the Store's ConsolidationFunc.
func (a *App) mergeMemoriesLLM(ctx context.Context, content1, content2 string) (string, error) {
	a.providerMu.RLock()
	router := a.providerRouter
	a.providerMu.RUnlock()
	if router == nil {
		return "", fmt.Errorf("no provider router available")
	}
	req := provider.ChatRequest{
		MaxTokens: 200,
		Messages: []provider.Message{
			provider.TextMessage("system",
				"You are a memory consolidation assistant. Given two similar memory entries, "+
					"create ONE concise memory that preserves all important information from both. "+
					"Output ONLY the merged memory text — no explanations, no labels, no prefixes."),
			provider.TextMessage("user",
				fmt.Sprintf("Memory 1: %s\n\nMemory 2: %s\n\nMerge into one memory:", content1, content2)),
		},
	}
	resp, err := router.ChatCompletion(ctx, req)
	if err != nil {
		return "", fmt.Errorf("merge LLM call: %w", err)
	}
	return strings.TrimSpace(resp.Content), nil
}

// SetMemoryEnabled toggles the memory feature.
// When enabling, EmbeddingAutoStart is persisted so the embedding model
// auto-starts on the next restart as well. If the embedding server is not
// already running and the model repo/file are configured, the download-and-
// start sequence runs immediately in the background.
func (a *App) SetMemoryEnabled(enabled bool) error {
	a.cfgMu.Lock()
	a.cfg.Memory.MemoryEnabled = enabled
	if enabled {
		a.cfg.Memory.EmbeddingAutoStart = true
	}
	a.cfgMu.Unlock()
	if err := config.Save(a.cfg); err != nil {
		return err
	}
	if enabled &&
		a.cfg.Memory.EmbeddingModelRepo != "" &&
		a.cfg.Memory.EmbeddingModelFile != "" &&
		!a.llamaEmbedServer.IsRunning() {
		go a.startupEmbeddingModel()
	}
	return nil
}

// SaveExplicitMemory saves a user-provided memory entry.
func (a *App) SaveExplicitMemory(content, tags string) error {
	a.storeMu.Lock()
	defer a.storeMu.Unlock()
	if a.store == nil {
		return fmt.Errorf("memory store not initialized")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	return a.store.SaveExplicit(ctx, content, tags)
}

// DeleteExplicitMemory deletes memories matching the given pattern.
func (a *App) DeleteExplicitMemory(pattern string) (int, error) {
	a.storeMu.Lock()
	defer a.storeMu.Unlock()
	if a.store == nil {
		return 0, fmt.Errorf("memory store not initialized")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return a.store.DeleteByContent(ctx, pattern)
}

// ExportMemories exports all memories as JSON bytes.
func (a *App) ExportMemories() ([]byte, error) {
	a.storeMu.RLock()
	defer a.storeMu.RUnlock()
	if a.store == nil {
		return nil, fmt.Errorf("memory store not initialized")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	return a.store.Export(ctx)
}

// ImportMemories imports memories from JSON bytes.
func (a *App) ImportMemories(data []byte) (int, error) {
	a.storeMu.RLock()
	defer a.storeMu.RUnlock()
	if a.store == nil {
		return 0, fmt.Errorf("memory store not initialized")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	return a.store.Import(ctx, data)
}

func (a *App) GetMemoryStats() models.MemoryStats {
	a.storeMu.RLock()
	defer a.storeMu.RUnlock()
	if a.store == nil {
		return models.MemoryStats{}
	}
	return a.store.Stats()
}

func (a *App) FilteredMemorySearch(query string, topK int, since string, tag string) []memory.MemoryResult {
	a.storeMu.RLock()
	defer a.storeMu.RUnlock()
	if a.store == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	var sinceTime time.Time
	if since != "" {
		if t, err := time.Parse(time.RFC3339, since); err == nil {
			sinceTime = t
		}
	}
	results, err := a.store.FilteredSearch(ctx, query, topK, a.cfg.Memory.MinSimilarity, sinceTime, tag)
	if err != nil {
		logx.Printf("MEMORY: filtered search: %v", err)
		return nil
	}
	return results
}

// CheckEmbeddingHealth tests if the embedding API is reachable and working.
func (a *App) CheckEmbeddingHealth(ctx context.Context) map[string]interface{} {
	result := map[string]interface{}{
		"ok":    false,
		"error": "",
		"count": 0,
	}

	a.storeMu.RLock()
	defer a.storeMu.RUnlock()

	if a.store == nil {
		result["error"] = "memory store not initialized"
		return result
	}

	result["count"] = a.store.Count()

	a.clientMu.RLock()
	client := a.client
	if a.embeddingClient != nil {
		client = a.embeddingClient
	}
	a.clientMu.RUnlock()

	ectx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	_, err := client.CreateEmbedding(ectx, a.cfg.API.EmbeddingModel, "test")
	if err != nil {
		result["error"] = err.Error()
		logx.Printf("EMBEDDING HEALTH CHECK FAILED: %v", err)
		return result
	}

	result["ok"] = true
	logx.Printf("Embedding health: OK (model=%s, memories=%d)", a.cfg.API.EmbeddingModel, a.store.Count())
	return result
}
