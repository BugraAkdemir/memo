package app

import (
	"fmt"
	"memo/internal/logx"
	"time"

	"memo/internal/modelstore"
)

// SearchModels searches HuggingFace for GGUF models matching the query.
func (a *App) SearchModels(query string) ([]modelstore.HFModelResult, error) {
	results, err := a.modelStore.SearchModels(query)
	if err != nil {
		logx.Printf("SearchModels error: %v", err)
		return nil, fmt.Errorf("search failed: %w", err)
	}
	return results, nil
}

// GetModelFiles returns the GGUF files available in a HuggingFace repo.
func (a *App) GetModelFiles(repoID string) []modelstore.GGUFFile {
	files, err := a.modelStore.GetModelFiles(repoID)
	if err != nil {
		logx.Printf("GetModelFiles error: %v", err)
		return nil
	}
	return files
}

// DownloadModel downloads a model file from HuggingFace. expectedSize (from
// HF API tree endpoint) is used as a fallback when the CDN serves the file
// with chunked transfer encoding (no Content-Length header).
func (a *App) DownloadModel(repoID, filename string, expectedSize int64) error {
	if err := a.modelStore.DownloadModel(repoID, filename, expectedSize); err != nil {
		return err
	}
	// The download itself runs on modelstore's own background goroutine, so
	// this call already returned before it finishes. Watch for it to land
	// and auto-start embedding then if needed — otherwise the common
	// first-run case (backend starts before any model is downloaded, so the
	// Startup()-time auto-start finds nothing) would leave embedding/RAG
	// dead for the rest of the session even after downloading one, until
	// the user finds /embedding.
	go a.autoStartEmbeddingAfterDownload()
	return nil
}

// autoStartEmbeddingAfterDownload waits for the in-flight download to finish,
// then starts an embedding model automatically if memory is enabled and
// nothing is running yet. Safe to call even if the just-downloaded file
// wasn't an embedding model — autoStartEmbeddingModel re-scans all local
// models itself.
func (a *App) autoStartEmbeddingAfterDownload() {
	if !a.cfg.Memory.MemoryEnabled {
		return
	}
	for range 600 { // up to 5 minutes — large GGUF files take a while
		time.Sleep(500 * time.Millisecond)
		p := a.modelStore.GetDownloadProgress()
		if p == nil || !p.Active {
			break
		}
	}
	if a.llamaEmbedServer != nil && a.llamaEmbedServer.IsRunning() {
		return
	}
	a.autoStartEmbeddingModel()
}

// GetDownloadProgress returns the current download progress.
func (a *App) GetDownloadProgress() *modelstore.DownloadProgress {
	return a.modelStore.GetDownloadProgress()
}

// CancelDownload cancels an in-progress model download.
func (a *App) CancelDownload() {
	a.modelStore.CancelDownload()
}

// ImportLocalModel copies a local GGUF file into the models directory. Unlike
// DownloadModel this finishes synchronously, so if it's an embedding model
// and nothing is running yet, auto-start can happen right here.
func (a *App) ImportLocalModel(sourcePath string) error {
	if err := a.modelStore.ImportLocalModel(sourcePath); err != nil {
		return err
	}
	if a.cfg.Memory.MemoryEnabled && (a.llamaEmbedServer == nil || !a.llamaEmbedServer.IsRunning()) {
		go a.autoStartEmbeddingModel()
	}
	return nil
}

// ListLocalModels returns all locally available models.
func (a *App) ListLocalModels() []modelstore.LocalModel {
	return a.modelStore.ListLocalModels()
}

// DeleteLocalModel removes a local model file.
func (a *App) DeleteLocalModel(path string) error {
	return a.modelStore.DeleteLocalModel(path)
}
