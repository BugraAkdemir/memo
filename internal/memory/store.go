package memory

import (
	"context"
	"crypto/sha256"
	"encoding/gob"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	chromem "github.com/philippgille/chromem-go"
)

const (
	collectionName   = "conversations"
	metadataFileName = "00000000.gob"
)

type MemoryIndex struct {
	ID     string
	Vector []float32
	Norm   float64 // pre-computed L2 norm for faster cosine similarity
}

type Store struct {
	mu            sync.RWMutex
	index         []MemoryIndex
	persistDir    string
	collectionDir string
	embeddingFunc chromem.EmbeddingFunc
	indexPath     string // single-file index path for fast startup
}

func NewStore(persistDir string, embeddingFunc chromem.EmbeddingFunc) (*Store, error) {
	if persistDir == "" {
		persistDir = "./chromem-go"
	}
	if embeddingFunc == nil {
		return nil, fmt.Errorf("memory.NewStore: embeddingFunc is nil")
	}

	s := &Store{
		persistDir:    filepath.Clean(persistDir),
		collectionDir: filepath.Join(filepath.Clean(persistDir), hash2hex(collectionName)),
		embeddingFunc: embeddingFunc,
		indexPath:     filepath.Join(filepath.Clean(persistDir), "memory_index.gob"),
	}

	if err := s.ensureCollectionMetadata(); err != nil {
		return nil, fmt.Errorf("memory.NewStore: %w", err)
	}
	if err := s.LoadCache(); err != nil {
		return nil, fmt.Errorf("memory.NewStore: load cache: %w", err)
	}

	return s, nil
}

func (s *Store) LoadCache() error {
	start := time.Now()

	// Try single-file index first (fast startup).
	nextIndex, err := s.loadIndexFile()
	if err == nil {
		s.mu.Lock()
		s.index = nextIndex
		s.mu.Unlock()
		log.Printf("LATENCY memory.load_cache total_ms=%d indexed=%d (fast path)", time.Since(start).Milliseconds(), len(nextIndex))
		return nil
	}

	// Fallback: scan individual .gob files (legacy path, slow).
	entries, err := os.ReadDir(s.collectionDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			s.mu.Lock()
			s.index = nil
			s.mu.Unlock()
			return nil
		}
		return fmt.Errorf("read collection dir: %w", err)
	}

	nextIndex = make([]MemoryIndex, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || entry.Name() == metadataFileName || filepath.Ext(entry.Name()) != ".gob" {
			continue
		}

		doc, err := readDocument(filepath.Join(s.collectionDir, entry.Name()))
		if err != nil {
			return fmt.Errorf("read index document %q: %w", entry.Name(), err)
		}
		if doc.ID == "" || len(doc.Embedding) == 0 {
			continue
		}

		nextIndex = append(nextIndex, MemoryIndex{
			ID:     doc.ID,
			Vector: doc.Embedding,
			Norm:   vectorNorm(doc.Embedding),
		})
	}

	// Persist the rebuilt index for next startup.
	if err := s.writeIndexFile(nextIndex); err != nil {
		log.Printf("memory: failed to persist index file: %v", err)
	}

	s.mu.Lock()
	s.index = nextIndex
	s.mu.Unlock()
	log.Printf("LATENCY memory.load_cache total_ms=%d indexed=%d (legacy path)", time.Since(start).Milliseconds(), len(nextIndex))
	return nil
}

// loadIndexFile reads the single-file index from disk.
func (s *Store) loadIndexFile() ([]MemoryIndex, error) {
	f, err := os.Open(s.indexPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var idx []MemoryIndex
	if err := gob.NewDecoder(f).Decode(&idx); err != nil {
		return nil, err
	}
	return idx, nil
}

// writeIndexFile persists the in-memory index as a single gob file.
func (s *Store) writeIndexFile(idx []MemoryIndex) error {
	f, err := os.Create(s.indexPath)
	if err != nil {
		return err
	}
	defer f.Close()
	return gob.NewEncoder(f).Encode(idx)
}

func (s *Store) SaveInteraction(ctx context.Context, userMsg, assistantMsg string) error {
	totalStart := time.Now()
	now := time.Now()
	ts := now.Format(time.RFC3339)

	embedStart := time.Now()
	embedding, err := s.embeddingFunc(ctx, userMsg)
	if err != nil {
		return fmt.Errorf("memory.SaveInteraction: embed: %w", err)
	}
	embedDuration := time.Since(embedStart)

	content := fmt.Sprintf("[%s] User: %s\nAssistant: %s", ts, userMsg, assistantMsg)
	doc := chromem.Document{
		ID:        fmt.Sprintf("conv_%d", now.UnixNano()),
		Content:   content,
		Embedding: normalizeVector(embedding),
		Metadata: map[string]string{
			"timestamp":  ts,
			"type":       "conversation",
			"user_msg":   truncate(userMsg, 200),
			"assist_msg": truncate(assistantMsg, 200),
		},
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	diskStart := time.Now()
	if err := writeDocument(s.documentPath(doc.ID), doc); err != nil {
		return fmt.Errorf("memory.SaveInteraction: persist: %w", err)
	}
	diskDuration := time.Since(diskStart)

	vec := append([]float32(nil), doc.Embedding...)
	idx := MemoryIndex{
		ID:     doc.ID,
		Vector: vec,
		Norm:   vectorNorm(vec),
	}
	s.upsertIndexLocked(idx)
	if err := s.writeIndexFile(s.index); err != nil {
		log.Printf("memory: failed to persist index: %v", err)
	}
	log.Printf(
		"LATENCY memory.save total_ms=%d embed_ms=%d disk_ms=%d cache_docs=%d",
		time.Since(totalStart).Milliseconds(),
		embedDuration.Milliseconds(),
		diskDuration.Milliseconds(),
		len(s.index),
	)
	return nil
}

func (s *Store) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.index)
}

// ClearAll removes all memory by deleting the persist directory and reinitializing.
func (s *Store) ClearAll() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := os.RemoveAll(s.persistDir); err != nil {
		return fmt.Errorf("memory.ClearAll: %w", err)
	}
	if err := s.ensureCollectionMetadataLocked(); err != nil {
		return fmt.Errorf("memory.ClearAll: reinit metadata: %w", err)
	}

	s.index = nil
	return nil
}

// ListGobFiles returns all .gob files with their paths and sizes for selective deletion.
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
		if !info.IsDir() && filepath.Ext(path) == ".gob" && info.Name() != metadataFileName && info.Name() != "memory_index.gob" {
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

// DeleteGobFile deletes a specific .gob memory file and removes it from the RAM index.
func (s *Store) DeleteGobFile(relPath string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	fullPath, err := s.safePersistPath(relPath)
	if err != nil {
		return err
	}
	if filepath.Ext(fullPath) != ".gob" || filepath.Base(fullPath) == metadataFileName {
		return fmt.Errorf("not a memory .gob file")
	}

	// Look up the document ID from the index using the filename hash,
	// so we don't need to read (possibly corrupted) gob files.
	baseName := strings.TrimSuffix(filepath.Base(fullPath), ".gob")
	var docID string
	for _, idx := range s.index {
		if hash2hex(idx.ID) == baseName {
			docID = idx.ID
			break
		}
	}

	// Delete the file; if it fails, keep the index intact.
	if err := os.Remove(fullPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("memory.DeleteGobFile: %w", err)
	}

	// Remove from index so stale entries don't accumulate (C9).
	if docID != "" {
		s.removeIndexLocked(docID)
	}
	return nil
}

func (s *Store) ensureCollectionMetadata() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.ensureCollectionMetadataLocked()
}

func (s *Store) ensureCollectionMetadataLocked() error {
	if err := os.MkdirAll(s.collectionDir, 0o700); err != nil {
		return err
	}

	path := filepath.Join(s.collectionDir, metadataFileName)
	if _, err := os.Stat(path); err == nil {
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}

	metadata := struct {
		Name     string
		Metadata map[string]string
	}{
		Name:     collectionName,
		Metadata: nil,
	}
	return writeGobFile(path, metadata)
}

func (s *Store) documentPath(id string) string {
	return filepath.Join(s.collectionDir, hash2hex(id)+".gob")
}

func (s *Store) safePersistPath(relPath string) (string, error) {
	if filepath.IsAbs(relPath) {
		return "", fmt.Errorf("absolute paths are not allowed")
	}

	base, err := filepath.Abs(s.persistDir)
	if err != nil {
		return "", err
	}
	full, err := filepath.Abs(filepath.Join(s.persistDir, relPath))
	if err != nil {
		return "", err
	}
	if full != base && !strings.HasPrefix(full, base+string(os.PathSeparator)) {
		return "", fmt.Errorf("path escapes memory directory")
	}
	// Resolve symlinks to prevent TOCTOU symlink swaps.
	resolved, err := filepath.EvalSymlinks(full)
	if err != nil {
		return "", err
	}
	if resolved != base && !strings.HasPrefix(resolved, base+string(os.PathSeparator)) {
		return "", fmt.Errorf("path escapes memory directory (symlink resolved)")
	}
	return resolved, nil
}

func (s *Store) upsertIndexLocked(item MemoryIndex) {
	for i := range s.index {
		if s.index[i].ID == item.ID {
			s.index[i] = item
			return
		}
	}
	s.index = append(s.index, item)
}

func (s *Store) removeIndexLocked(id string) {
	for i := range s.index {
		if s.index[i].ID == id {
			s.index = append(s.index[:i], s.index[i+1:]...)
			s.writeIndexFile(s.index)
			return
		}
	}
}

func readDocument(path string) (chromem.Document, error) {
	var doc chromem.Document
	if err := readGobFile(path, &doc); err != nil {
		return chromem.Document{}, err
	}
	return doc, nil
}

func writeDocument(path string, doc chromem.Document) error {
	return writeGobFile(path, doc)
}

func readGobFile(path string, v any) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	if err := gob.NewDecoder(f).Decode(v); err != nil {
		return err
	}
	return nil
}

func writeGobFile(path string, v any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}

	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	if err := gob.NewEncoder(f).Encode(v); err != nil {
		return err
	}
	return nil
}

func hash2hex(name string) string {
	hash := sha256.Sum256([]byte(name))
	return hex.EncodeToString(hash[:8])
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
