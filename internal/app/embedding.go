package app

import (
	"context"
	"errors"
	"fmt"
	"log"
	"memo/internal/config"
	"memo/internal/logx"
	"os"
	"path/filepath"
	"time"

	"memo/internal/api"
	"memo/internal/llama"
)

// StartEmbeddingModel is an explicit user request. Remember the choice so a
// successful manual start also opts in to startup and crash recovery.
func (a *App) StartEmbeddingModel(modelPath string, gpuLayers int) error {
	if a.modelStore != nil {
		for _, m := range a.modelStore.ListLocalModels() {
			if m.Path == modelPath && !m.IsEmbedding {
				return fmt.Errorf("chat models cannot be started as embedding models")
			}
		}
	}
	a.cfgMu.Lock()
	if !a.cfg.Memory.MemoryEnabled || a.embeddingPaused {
		a.cfgMu.Unlock()
		return errors.New("memory is disabled or the engine is being updated")
	}
	a.cfg.Memory.EmbeddingAutoStart = true
	err := config.Save(a.cfg)
	a.cfgMu.Unlock()
	if err != nil {
		return err
	}
	a.embeddingMu.Lock()
	defer a.embeddingMu.Unlock()
	return a.startEmbeddingLocked(modelPath, gpuLayers)
}

// Caller holds embeddingMu. The context is canceled by memory-off, manual stop
// or shutdown, including while downloading a first-run model or waiting for it.
func (a *App) embeddingContextLocked() (context.Context, context.CancelFunc, bool) {
	a.cfgMu.Lock()
	defer a.cfgMu.Unlock()
	if !a.cfg.Memory.MemoryEnabled || !a.cfg.Memory.EmbeddingAutoStart || a.embeddingPaused {
		return nil, nil, false
	}
	parent := a.lifecycleCtx
	if parent == nil {
		parent = context.Background()
	}
	ctx, cancel := context.WithCancel(parent)
	a.embeddingCancel = cancel
	return ctx, func() {
		cancel()
		a.cfgMu.Lock()
		a.embeddingCancel = nil
		a.cfgMu.Unlock()
	}, true
}

func (a *App) startEmbeddingLocked(modelPath string, gpuLayers int) error {
	ctx, done, ok := a.embeddingContextLocked()
	if !ok {
		return context.Canceled
	}
	defer done()
	return a.launchEmbeddingLocked(ctx, modelPath, gpuLayers)
}

func (a *App) launchEmbeddingLocked(ctx context.Context, modelPath string, gpuLayers int) error {
	if a.memoryStoreReady != nil {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-a.memoryStoreReady:
		}
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if a.llamaEmbedServer.IsRunning() {
		a.llamaEmbedServer.Stop()
		time.Sleep(500 * time.Millisecond)
	}

	a.cfgMu.RLock()
	embPort := a.cfg.Llama.EmbeddingPort
	if embPort <= 0 || embPort == a.cfg.Llama.Port {
		embPort = 8082
	}
	binaryPath := a.cfg.Llama.BinaryPath
	engineMode := a.cfg.Llama.EngineMode
	a.cfgMu.RUnlock()

	const maxAttempts = 3
	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		logx.Printf("Starting embedding model on port %d (attempt %d/%d)", embPort, attempt, maxAttempts)

		if err := a.llamaEmbedServer.Start(binaryPath, modelPath, 512, embPort, gpuLayers, true, engineMode); err != nil {
			noteLocalStartFail("embedding", err)
			return err
		}

		if err := a.llamaEmbedServer.WaitReadyContext(ctx, 120*time.Second); err != nil {
			a.llamaEmbedServer.Stop()
			if ctx.Err() != nil {
				return ctx.Err()
			}
			lastErr = err
			if attempt < maxAttempts {
				logx.Printf("embedding model failed to start (attempt %d/%d): %v — retrying", attempt, maxAttempts, err)
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(time.Second):
				}
				continue
			}
			noteLocalStartFail("embedding", lastErr) // once per request, not once per retry
			return fmt.Errorf("embedding model loaded but server failed to start after %d attempts: %w", maxAttempts, lastErr)
		}
		break
	}

	if err := ctx.Err(); err != nil {
		_ = a.llamaEmbedServer.Stop()
		return err
	}
	a.embeddingPath, a.embeddingLayers = modelPath, gpuLayers
	embBaseURL := a.llamaEmbedServer.GetBaseURL()
	a.cfgMu.RLock()
	timeoutSeconds, embeddingModel := a.cfg.API.TimeoutSeconds, a.cfg.API.EmbeddingModel
	a.cfgMu.RUnlock()
	a.clientMu.Lock()
	a.embeddingClient = api.NewClient(embBaseURL, timeoutSeconds)
	embClient := a.embeddingClient
	a.clientMu.Unlock()

	a.reinitMemoryStore(embClient, embeddingModel)
	logx.Printf("Embedding server ready on %s", embBaseURL)

	return nil
}

// StopEmbeddingModel stops the dedicated embedding llama-server.
func (a *App) StopEmbeddingModel() error {
	a.cfgMu.Lock()
	a.cfg.Memory.EmbeddingAutoStart = false
	if a.embeddingCancel != nil {
		a.embeddingCancel()
	}
	err := config.Save(a.cfg)
	a.cfgMu.Unlock()
	a.embeddingMu.Lock()
	defer a.embeddingMu.Unlock()
	return errors.Join(err, a.stopEmbeddingLocked())
}

// Internal stops (memory off, engine update) preserve the user's startup choice.
func (a *App) stopEmbeddingLocked() error {
	if err := a.llamaEmbedServer.Stop(); err != nil {
		return err
	}

	a.clientMu.Lock()
	a.embeddingClient = nil
	a.clientMu.Unlock()
	logx.Info("Embedding server stopped")

	a.clientMu.RLock()
	mainClient := a.client
	a.clientMu.RUnlock()
	a.reinitMemoryStore(mainClient, a.cfg.API.EmbeddingModel)

	return nil
}

// GetEmbeddingModelStatus returns the current embedding server status.
func (a *App) GetEmbeddingModelStatus() llama.ServerStatus {
	return a.llamaEmbedServer.GetStatus()
}

// reconnectEmbeddingIfAlreadyRunning wires the memory store to an embedding
// server this process doesn't have tracked (a.llamaEmbedServer.IsRunning()
// == false) but that's actually alive on the configured port — detected the
// same way GetStatus() already reports it as "running" for display (its own
// pingPort() fallback), except that call is read-only and never reconnects
// anything. Left at whatever placeholder (a.client-based) embedding
// function Startup() wired up otherwise, so memory silently embeds through
// the wrong model until something explicitly calls StartEmbeddingModel.
//
// Deliberately does not go through Start() — that calls killByPort first,
// which would kill a perfectly good, already-running server just to
// relaunch an identical one.
func (a *App) reconnectEmbeddingIfAlreadyRunning() {
	if a.llamaEmbedServer.IsRunning() {
		return // already tracked by this process — nothing to reconnect
	}
	if !a.llamaEmbedServer.GetStatus().Running {
		return // nothing answering on the configured port
	}

	a.cfgMu.RLock()
	timeoutSeconds := a.cfg.API.TimeoutSeconds
	embeddingModel := a.cfg.API.EmbeddingModel
	a.cfgMu.RUnlock()

	baseURL := a.llamaEmbedServer.GetBaseURL()
	logx.Printf("MEMORY: found an already-running embedding server on %s, reconnecting memory store to it", baseURL)
	embClient := api.NewClient(baseURL, timeoutSeconds)
	a.clientMu.Lock()
	a.embeddingClient = embClient
	a.clientMu.Unlock()
	a.reinitMemoryStore(embClient, embeddingModel)
}

// superviseEmbedding retries at a fixed, bounded rate, never in a hot crash loop.
// The same settings gate boot, downloads, local-model hooks and crash recovery.
func (a *App) superviseEmbedding(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		if ctx.Err() != nil {
			return
		}
		a.startupEmbeddingModel()
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (a *App) startupEmbeddingModel() {
	a.embeddingMu.Lock()
	defer a.embeddingMu.Unlock()
	ctx, done, ok := a.embeddingContextLocked()
	if !ok {
		return
	}
	defer done()
	if ctx.Err() != nil {
		return
	}
	if a.llamaEmbedServer.GetStatus().Running {
		a.clientMu.RLock()
		connected := a.embeddingClient != nil
		a.clientMu.RUnlock()
		if !connected {
			a.reconnectEmbeddingIfAlreadyRunning()
		}
		return
	}
	a.cfgMu.RLock()
	repoID := a.cfg.Memory.EmbeddingModelRepo
	filename := a.cfg.Memory.EmbeddingModelFile
	modelsDir := a.cfg.Llama.ModelsDir
	embGPULayers := a.cfg.Llama.EmbeddingGPULayers
	a.cfgMu.RUnlock()
	modelPath := a.embeddingPath
	if modelPath != "" {
		embGPULayers = a.embeddingLayers
	} else {
		if filename == "" {
			return
		}
		modelPath = filepath.Join(modelsDir, filename)
	}
	if _, err := os.Stat(modelPath); os.IsNotExist(err) {
		// Imported/downloaded embedding models need not use the bundled default's filename.
		if a.embeddingPath == "" && a.modelStore != nil {
			for _, m := range a.modelStore.ListLocalModels() {
				if m.IsEmbedding {
					modelPath = m.Path
					break
				}
			}
		}
	}
	if _, err := os.Stat(modelPath); os.IsNotExist(err) {
		if repoID == "" || a.embeddingPath != "" {
			return
		}
		logx.Printf("Downloading embedding model: %s/%s ...", repoID, filename)
		a.emitEvent("memory:downloading", filename)
		if err := a.downloadFile(ctx, repoID, filename, modelPath); err != nil {
			if ctx.Err() == nil {
				logx.Printf("WARN: failed to download embedding model: %v", err)
			}
			return
		}
	}
	logx.Printf("Auto-starting embedding model: %s", modelPath)
	if err := a.launchEmbeddingLocked(ctx, modelPath, embGPULayers); err != nil {
		if ctx.Err() == nil {
			msg := fmt.Sprintf("Failed to start embedding model: %v", err)
			log.Print(msg)
			a.emitEvent("memory:error", msg)
		}
	} else {
		logx.Info("Cross-mode active: API provider for chat, local model for embeddings")
		a.emitEvent("memory:ready", filepath.Base(modelPath))
	}
}
