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

	embPort := a.cfg.Llama.EmbeddingPort
	if embPort <= 0 || embPort == a.cfg.Llama.Port {
		embPort = 8082
	}

	const maxAttempts = 3
	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		logx.Printf("Starting embedding model on port %d (attempt %d/%d)", embPort, attempt, maxAttempts)

		if err := a.llamaEmbedServer.Start(a.cfg.Llama.BinaryPath, modelPath, 512, embPort, gpuLayers, true, a.cfg.Llama.EngineMode); err != nil {
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

// startupEmbeddingModel ensures the embedding model is available and running at startup.
func (a *App) startupEmbeddingModel() {
	repoID := a.cfg.Memory.EmbeddingModelRepo
	filename := a.cfg.Memory.EmbeddingModelFile
	modelsDir := a.cfg.Llama.ModelsDir
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
