package app

import (
	"context"
	"fmt"
	"log"
	"time"

	"memo/internal/api"
	"memo/internal/config"
	"memo/internal/memory"
)

func (a *App) saveMemoryAsync(userMsg, reply string) {
	if reply == "" || !a.cfg.Memory.MemoryEnabled {
		return
	}
	select {
	case a.memorySaveCh <- saveTask{userMsg: userMsg, reply: reply}:
	default:
		log.Println("WARN: memory save channel full, dropping")
	}
}

func (a *App) memorySaveWorker() {
	for task := range a.memorySaveCh {
		a.saveMemorySync(a.lifecycleCtx, task.userMsg, task.reply)
	}
}

func (a *App) saveMemorySync(ctx context.Context, userMsg, reply string) {
	if !a.cfg.Memory.MemoryEnabled {
		return
	}
	start := time.Now()

	a.storeMu.Lock()
	defer a.storeMu.Unlock()
	if a.store == nil {
		log.Println("MEMORY SAVE SKIPPED: store not initialized")
		a.emitEvent("memory:error", "Hafıza kaydedilemedi: depo başlatılmamış")
		return
	}

	mctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	if err := a.store.SaveInteraction(mctx, userMsg, reply); err != nil {
		log.Printf("LATENCY app.memory_save_sync total_ms=%d status=error", time.Since(start).Milliseconds())
		log.Printf("MEMORY SAVE FAILED: %v", err)
		a.emitEvent("memory:error", fmt.Sprintf("Hafıza kaydedilemedi: %v", err))
	} else {
		log.Printf("LATENCY app.memory_save_sync total_ms=%d status=ok", time.Since(start).Milliseconds())
		log.Printf("Memory saved: %q → %d chars reply", truncateLog(userMsg, 60), len(reply))
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
		log.Println("Memory: store not initialized, skipping retrieve")
		return nil
	}
	start := time.Now()
	rctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	m, err := a.store.RetrieveContext(rctx, query, a.cfg.Memory.TopK, a.cfg.Memory.MinSimilarity)
	if err != nil {
		log.Printf("LATENCY app.retrieve_memory total_ms=%d status=error", time.Since(start).Milliseconds())
		log.Printf("MEMORY RETRIEVE FAILED: %v", err)
		a.emitEvent("memory:error", fmt.Sprintf("Hafıza okunamadı: %v", err))
		return nil
	}
	log.Printf("LATENCY app.retrieve_memory total_ms=%d returned=%d", time.Since(start).Milliseconds(), len(m))
	if len(m) > 0 {
		log.Printf("Memory: found %d relevant memories (best=%.0f%%)", len(m), m[0].Similarity*100)
	}
	return m
}

func (a *App) reinitMemoryStore(client *api.Client, model string) {
	// Close the old store first so it releases the SQLite write lock
	// before NewStore opens the same file. Opening both simultaneously
	// causes the migration write to block until the old connection's
	// WAL lock is released, which can take up to the migration timeout.
	a.storeMu.Lock()
	if a.store != nil {
		if err := a.store.Close(); err != nil {
			log.Printf("WARN: memory store close: %v", err)
		}
		a.store = nil
	}
	a.storeMu.Unlock()

	embeddingFunc := memory.NewEmbeddingFunc(client, model)
	newStore, err := memory.NewStore(memory.StoreConfig{
		Dir:           a.cfg.Memory.PersistDir,
		Dimension:     a.cfg.Memory.EmbeddingDimension,
		EmbeddingFunc: embeddingFunc,
	})
	if err != nil {
		log.Printf("WARN: memory re-init: %v", err)
		a.emitEvent("memory_store_error", err.Error())
		return
	}
	a.storeMu.Lock()
	defer a.storeMu.Unlock()
	a.store = newStore
	log.Println("Memory store re-initialized")
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
	log.Println("Clearing all memory...")
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
	log.Printf("Deleting memory file: %s", relPath)
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
	log.Printf("Memory settings updated: top_k=%d min_similarity=%.2f", topK, minSimilarity)
	return nil
}

// GetMemoryEnabled reports whether memory is enabled.
func (a *App) GetMemoryEnabled() bool {
	return a.cfg.Memory.MemoryEnabled
}

// SetMemoryEnabled toggles the memory feature.
func (a *App) SetMemoryEnabled(enabled bool) error {
	a.cfg.Memory.MemoryEnabled = enabled
	return config.Save(a.cfg)
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
		log.Printf("EMBEDDING HEALTH CHECK FAILED: %v", err)
		return result
	}

	result["ok"] = true
	log.Printf("Embedding health: OK (model=%s, memories=%d)", a.cfg.API.EmbeddingModel, a.store.Count())
	return result
}
