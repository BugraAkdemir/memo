package app

import (
	"fmt"
	"log"
	"memo/internal/logx"
	"os"
	"path/filepath"
	"time"

	"memo/internal/api"
	"memo/internal/llama"
)

// StartEmbeddingModel starts the dedicated embedding llama-server. The
// freshly launched process occasionally dies within its first second or two
// (observed as an unexplained external SIGTERM moments after a clean start —
// not something Stop()/killPID/killByPort in this codebase ever triggers,
// since those all mark the server as stopping first and this exit is always
// logged as unexpected). A single retry recovers from that transient case
// instead of leaving memory silently off until the user manually reruns
// /embedding.
func (a *App) StartEmbeddingModel(modelPath string, gpuLayers int) error {
	if a.modelStore != nil {
		for _, m := range a.modelStore.ListLocalModels() {
			if m.Path == modelPath && !m.IsEmbedding {
				return fmt.Errorf("sohbet modeli Hafıza (Embedding) Modeli olarak başlatılamaz")
			}
		}
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
		logx.Printf("Starting embedding model on port %d (attempt %d/%d)", embPort, attempt, maxAttempts)

		if err := a.llamaEmbedServer.Start(binaryPath, modelPath, 512, embPort, gpuLayers, true, engineMode); err != nil {
			return err
		}

		if err := a.llamaEmbedServer.WaitReady(120 * time.Second); err != nil {
			a.llamaEmbedServer.Stop()
			lastErr = err
			if attempt < maxAttempts {
				logx.Printf("embedding model failed to start (attempt %d/%d): %v — retrying", attempt, maxAttempts, err)
				time.Sleep(1 * time.Second)
				continue
			}
			return fmt.Errorf("embedding model loaded but server failed to start after %d attempts: %w", maxAttempts, lastErr)
		}
		break
	}

	embBaseURL := a.llamaEmbedServer.GetBaseURL()
	a.clientMu.Lock()
	a.embeddingClient = api.NewClient(embBaseURL, a.cfg.API.TimeoutSeconds)
	embClient := a.embeddingClient
	a.clientMu.Unlock()

	a.reinitMemoryStore(embClient, a.cfg.API.EmbeddingModel)
	logx.Printf("Embedding server ready on %s", embBaseURL)

	return nil
}

// StopEmbeddingModel stops the dedicated embedding llama-server.
func (a *App) StopEmbeddingModel() error {
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

// startupEmbeddingModel ensures the embedding model is available and running at startup.
func (a *App) startupEmbeddingModel() {
	repoID := a.cfg.Memory.EmbeddingModelRepo
	filename := a.cfg.Memory.EmbeddingModelFile
	a.cfgMu.RLock()
	modelsDir := a.cfg.Llama.ModelsDir
	a.cfgMu.RUnlock()
	modelPath := filepath.Join(modelsDir, filename)

	if _, err := os.Stat(modelPath); os.IsNotExist(err) {
		logx.Printf("Downloading embedding model: %s/%s ...", repoID, filename)
		a.emitEvent("memory:downloading", filename)
		if err := a.downloadFile(repoID, filename, modelPath); err != nil {
			logx.Printf("WARN: failed to download embedding model: %v", err)
			a.emitEvent("memory:error", fmt.Sprintf("Embedding model indirme hatası: %v", err))
			return
		}
		logx.Printf("Embedding model downloaded: %s", modelPath)
	}

	logx.Printf("Auto-starting embedding model: %s", modelPath)
	if err := a.StartEmbeddingModel(modelPath, -1); err != nil {
		msg := fmt.Sprintf("Failed to start embedding model: %v", err)
		log.Print(msg)
		a.emitEvent("memory:error", msg)
	} else {
		logx.Info("Cross-mode active: API provider for chat, local model for embeddings")
		a.emitEvent("memory:ready", filename)
	}
}
