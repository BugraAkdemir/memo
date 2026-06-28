package app

import (
	"fmt"
	"memo/internal/logx"

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

// DownloadModel downloads a model file from HuggingFace.
func (a *App) DownloadModel(repoID, filename string) error {
	return a.modelStore.DownloadModel(repoID, filename)
}

// GetDownloadProgress returns the current download progress.
func (a *App) GetDownloadProgress() *modelstore.DownloadProgress {
	return a.modelStore.GetDownloadProgress()
}

// CancelDownload cancels an in-progress model download.
func (a *App) CancelDownload() {
	a.modelStore.CancelDownload()
}

// ImportLocalModel copies a local GGUF file into the models directory.
func (a *App) ImportLocalModel(sourcePath string) error {
	return a.modelStore.ImportLocalModel(sourcePath)
}

// ListLocalModels returns all locally available models.
func (a *App) ListLocalModels() []modelstore.LocalModel {
	return a.modelStore.ListLocalModels()
}

// DeleteLocalModel removes a local model file.
func (a *App) DeleteLocalModel(path string) error {
	return a.modelStore.DeleteLocalModel(path)
}
