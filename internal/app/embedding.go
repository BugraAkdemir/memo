package app

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"memo/internal/api"
	"memo/internal/llama"
)

// StartEmbeddingModel starts the dedicated embedding llama-server.
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
	log.Printf("Starting embedding model on port %d", embPort)

	if err := a.llamaEmbedServer.Start(a.cfg.Llama.BinaryPath, modelPath, 512, embPort, gpuLayers, true, a.cfg.Llama.EngineMode); err != nil {
		return err
	}

	if err := a.llamaEmbedServer.WaitReady(120 * time.Second); err != nil {
		a.llamaEmbedServer.Stop()
		return fmt.Errorf("embedding model loaded but server failed to start: %w", err)
	}

	embBaseURL := a.llamaEmbedServer.GetBaseURL()
	a.clientMu.Lock()
	a.embeddingClient = api.NewClient(embBaseURL, a.cfg.API.TimeoutSeconds)
	embClient := a.embeddingClient
	a.clientMu.Unlock()

	a.reinitMemoryStore(embClient, a.cfg.API.EmbeddingModel)
	log.Printf("Embedding server ready on %s", embBaseURL)

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
	log.Println("Embedding server stopped")

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
		log.Printf("Downloading embedding model: %s/%s ...", repoID, filename)
		a.emitEvent("memory:downloading", filename)
		if err := a.downloadFile(repoID, filename, modelPath); err != nil {
			log.Printf("WARN: failed to download embedding model: %v", err)
			a.emitEvent("memory:error", fmt.Sprintf("Embedding model indirme hatası: %v", err))
			return
		}
		log.Printf("Embedding model downloaded: %s", modelPath)
	}

	log.Printf("Auto-starting embedding model: %s", modelPath)
	if err := a.StartEmbeddingModel(modelPath, -1); err != nil {
		msg := fmt.Sprintf("Failed to start embedding model: %v", err)
		log.Print(msg)
		a.emitEvent("memory:error", msg)
	} else {
		log.Println("Cross-mode active: API provider for chat, local model for embeddings")
		a.emitEvent("memory:ready", filename)
	}
}
