package memory

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	chromem "github.com/philippgille/chromem-go"
)

type Store struct {
	db            *chromem.DB
	collection    *chromem.Collection
	mu            sync.RWMutex
	docCount      int
	persistDir    string
	embeddingFunc chromem.EmbeddingFunc
}

func NewStore(persistDir string, embeddingFunc chromem.EmbeddingFunc) (*Store, error) {
	db, err := chromem.NewPersistentDB(persistDir, false)
	if err != nil {
		return nil, fmt.Errorf("memory.NewStore: %w", err)
	}

	col, err := db.GetOrCreateCollection("conversations", nil, embeddingFunc)
	if err != nil {
		return nil, fmt.Errorf("memory.NewStore: create collection: %w", err)
	}

	return &Store{
		db:            db,
		collection:    col,
		docCount:      col.Count(),
		persistDir:    persistDir,
		embeddingFunc: embeddingFunc,
	}, nil
}

func (s *Store) SaveInteraction(ctx context.Context, userMsg, assistantMsg string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	ts := time.Now().Format(time.RFC3339)

	// Generate embedding from USER MESSAGE ONLY — this gives much better
	// retrieval accuracy. The full conversation is stored in Content for the LLM.
	embedding, err := s.embeddingFunc(ctx, userMsg)
	if err != nil {
		return fmt.Errorf("memory.SaveInteraction: embed: %w", err)
	}

	content := fmt.Sprintf("[%s] User: %s\nAssistant: %s", ts, userMsg, assistantMsg)

	doc := chromem.Document{
		ID:        fmt.Sprintf("conv_%d_%s", s.docCount+1, time.Now().Format("20060102_150405")),
		Content:   content,
		Embedding: embedding,
		Metadata: map[string]string{
			"timestamp":  ts,
			"type":       "conversation",
			"user_msg":   truncate(userMsg, 200),
			"assist_msg": truncate(assistantMsg, 200),
		},
	}

	if err := s.collection.AddDocuments(ctx, []chromem.Document{doc}, runtime.NumCPU()); err != nil {
		return fmt.Errorf("memory.SaveInteraction: %w", err)
	}

	s.docCount++
	return nil
}

func (s *Store) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	// Use actual collection count — docCount can drift after restart or reinit
	return s.collection.Count()
}

// ClearAll removes all memory by deleting the persist directory and reinitializing
func (s *Store) ClearAll() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Remove all files in persist dir
	if err := os.RemoveAll(s.persistDir); err != nil {
		return fmt.Errorf("memory.ClearAll: %w", err)
	}

	// Reinitialize
	db, err := chromem.NewPersistentDB(s.persistDir, false)
	if err != nil {
		return fmt.Errorf("memory.ClearAll: reinit db: %w", err)
	}

	col, err := db.GetOrCreateCollection("conversations", nil, s.embeddingFunc)
	if err != nil {
		return fmt.Errorf("memory.ClearAll: reinit col: %w", err)
	}

	s.db = db
	s.collection = col
	s.docCount = 0
	return nil
}

// ListGobFiles returns all .gob files with their paths and sizes for selective deletion
type GobFileInfo struct {
	Path     string `json:"path"`
	Name     string `json:"name"`
	SizeKB   int64  `json:"size_kb"`
	Modified string `json:"modified"`
}

func (s *Store) ListGobFiles() []GobFileInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var files []GobFileInfo
	filepath.Walk(s.persistDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if !info.IsDir() && filepath.Ext(path) == ".gob" && info.Name() != "_chromem.gob" {
			relPath, _ := filepath.Rel(s.persistDir, path)
			files = append(files, GobFileInfo{
				Path:     relPath,
				Name:     info.Name(),
				SizeKB:   info.Size() / 1024,
				Modified: info.ModTime().Format("2006-01-02 15:04"),
			})
		}
		return nil
	})
	return files
}

// DeleteGobFile deletes a specific .gob memory file
func (s *Store) DeleteGobFile(relPath string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	fullPath := filepath.Join(s.persistDir, relPath)

	// Safety: only delete .gob files within persistDir
	if filepath.Ext(fullPath) != ".gob" {
		return fmt.Errorf("not a .gob file")
	}

	if err := os.Remove(fullPath); err != nil {
		return fmt.Errorf("memory.DeleteGobFile: %w", err)
	}

	s.docCount--
	if s.docCount < 0 {
		s.docCount = 0
	}
	return nil
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
