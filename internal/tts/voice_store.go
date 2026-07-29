// Local, fully offline voice model catalog for Piper (Faz 2.6 — "TTS
// Store"). Unlike the external provider Router (provider.go/router.go),
// this needs no API key and makes no chat/synthesis call to a third party
// at all — it only downloads a static .onnx voice file (+ its .onnx.json
// sidecar) from Hugging Face once, after which Piper (tts.Synthesizer,
// Faz 1) runs entirely on-device. This is the piece that actually lets a
// user go from "no voice configured" to a working local voice without
// hand-placing a file themselves or configuring any external provider.
package tts

import (
	"context"
	"fmt"
	"io"
	"memo/internal/logx"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

// Voice identifies one downloadable Piper voice from the rhasspy/piper-voices
// Hugging Face repo. The repo's path convention (verified against real
// downloads during Faz 1's research, see PLAN_voice_live_mode_faz1.md's 1.1
// notes) is <lang>/<locale>/<name>/<quality>/<locale>-<name>-<quality>.onnx,
// e.g. tr/tr_TR/fahrettin/medium/tr_TR-fahrettin-medium.onnx.
type Voice struct {
	Locale  string `json:"locale"`  // e.g. "tr_TR", "en_US"
	Name    string `json:"name"`    // e.g. "fahrettin", "lessac"
	Quality string `json:"quality"` // e.g. "medium", "low", "high"
}

// ID is the stable identifier used in API requests and local filenames —
// matches the voice's own <locale>-<name>-<quality> base filename.
func (v Voice) ID() string {
	return v.Locale + "-" + v.Name + "-" + v.Quality
}

// Language is the two-letter language code the locale is scoped to
// ("tr_TR" -> "tr"), used both for the repo path and for grouping the
// curated list by language in the UI.
func (v Voice) Language() string {
	if i := strings.Index(v.Locale, "_"); i > 0 {
		return v.Locale[:i]
	}
	return v.Locale
}

func (v Voice) repoPath() string {
	base := v.ID()
	return fmt.Sprintf("%s/%s/%s/%s/%s.onnx", v.Language(), v.Locale, v.Name, v.Quality, base)
}

const piperVoicesRepo = "rhasspy/piper-voices"

// hfVoicesBaseURL is a var (not a const) so tests can point it at a local
// httptest server instead of making a real network call to Hugging Face.
var hfVoicesBaseURL = "https://huggingface.co/" + piperVoicesRepo + "/resolve/main/"

// CuratedVoices returns a small, hand-picked set of well-known voices —
// covering Memo's two primary UI languages (Turkish + English, see
// AGENTS.md's "Turkish + English mixed user-facing text is intentional")
// plus one alternate English voice. Not a full catalog of every voice the
// upstream repo hosts: browsing/searching the entire piper-voices tree is
// explicitly out of scope for this pass (see PLAN_voice_live_mode_faz2.md's
// 2.6 note) — a short curated list that's known to exist and work is a much
// smaller, more reliable piece than a general HF-tree browser, and covers
// the actual need (a user picks one voice, not many).
func CuratedVoices() []Voice {
	return []Voice{
		{Locale: "tr_TR", Name: "fahrettin", Quality: "medium"},
		{Locale: "en_US", Name: "lessac", Quality: "medium"},
		{Locale: "en_US", Name: "amy", Quality: "medium"},
	}
}

// LocalVoice describes a downloaded voice on disk.
type LocalVoice struct {
	Voice
	Path string `json:"path"`
	Size int64  `json:"size"`
}

// VoiceDownloadProgress tracks the state of an active or errored voice download.
type VoiceDownloadProgress struct {
	Active     bool    `json:"active"`
	VoiceID    string  `json:"voice_id"`
	TotalBytes int64   `json:"total_bytes"`
	Downloaded int64   `json:"downloaded"`
	Percent    float64 `json:"percent"`
	Error      string  `json:"error,omitempty"`
}

type voiceDownloadEntry struct {
	progress *VoiceDownloadProgress
	cancelFn context.CancelFunc
}

// VoiceStore manages downloading and listing local Piper voice models.
// Mirrors internal/modelstore.Store's download-tracking shape (Active/
// Downloaded/Percent, a goroutine per download, atomic rename on
// completion) trimmed down for voice files, which are small (tens of MB,
// not multi-gigabyte GGUFs) and need no GGUF-style sidecar metadata beyond
// Piper's own required .onnx.json.
type VoiceStore struct {
	voicesDir string
	client    *http.Client

	mu        sync.RWMutex
	downloads map[string]*voiceDownloadEntry
}

func NewVoiceStore(voicesDir string) *VoiceStore {
	if err := os.MkdirAll(voicesDir, 0755); err != nil {
		logx.Printf("tts.VoiceStore: cannot create voices dir: %v", err)
	}
	return &VoiceStore{
		voicesDir: voicesDir,
		client:    &http.Client{Timeout: 0},
		downloads: make(map[string]*voiceDownloadEntry),
	}
}

// DownloadVoice starts downloading v's .onnx model and .onnx.json sidecar
// in the background. Returns an error immediately only if this exact voice
// is already downloading.
func (s *VoiceStore) DownloadVoice(v Voice) error {
	id := v.ID()

	s.mu.Lock()
	if existing, ok := s.downloads[id]; ok && existing.progress.Active {
		s.mu.Unlock()
		return fmt.Errorf("tts: %s is already downloading", id)
	}
	ctx, cancel := context.WithCancel(context.Background())
	entry := &voiceDownloadEntry{
		cancelFn: cancel,
		progress: &VoiceDownloadProgress{Active: true, VoiceID: id},
	}
	s.downloads[id] = entry
	s.mu.Unlock()

	go func() {
		defer logx.Recover("tts.VoiceStore download: " + id)
		defer func() {
			s.mu.Lock()
			entry.cancelFn = nil
			s.mu.Unlock()
		}()

		if err := s.doDownload(ctx, entry, v); err != nil {
			s.mu.Lock()
			entry.progress.Error = err.Error()
			entry.progress.Active = true // keep visible so the UI can show the error
			s.mu.Unlock()
			logx.Printf("tts.VoiceStore: download failed for %s: %v", id, err)
			return
		}
		s.mu.Lock()
		entry.progress.Active = false
		entry.progress.Percent = 100
		s.mu.Unlock()
		logx.Printf("tts.VoiceStore: download complete: %s", id)
	}()

	return nil
}

func (s *VoiceStore) doDownload(ctx context.Context, entry *voiceDownloadEntry, v Voice) error {
	onnxPath := v.repoPath()
	destPath := filepath.Join(s.voicesDir, v.ID()+".onnx")
	if err := s.downloadFile(ctx, onnxPath, destPath, entry); err != nil {
		return fmt.Errorf("model: %w", err)
	}
	// The sidecar is small enough that a separate progress-tracked download
	// isn't worth it — Piper requires it to exist alongside the .onnx
	// (see resolveModel in tts.go), so a failure here must still surface
	// as a failure of the whole voice download, not a silently incomplete one.
	if err := s.downloadFile(ctx, onnxPath+".json", destPath+".json", nil); err != nil {
		os.Remove(destPath)
		return fmt.Errorf("sidecar: %w", err)
	}
	return nil
}

func (s *VoiceStore) downloadFile(ctx context.Context, repoPath, destPath string, entry *voiceDownloadEntry) error {
	// repoPath is built entirely from this package's own CuratedVoices()
	// entries (Locale/Name/Quality — never free-form user input), all of
	// which use only URL-safe characters (letters, digits, "_"), so no
	// escaping is needed beyond keeping the "/" path separators literal.
	downloadURL := hfVoicesBaseURL + repoPath

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, downloadURL, nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("User-Agent", "Memo-TTS-VoiceStore/1.0")

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("status %d: %s", resp.StatusCode, string(body))
	}

	if entry != nil {
		s.mu.Lock()
		entry.progress.TotalBytes = resp.ContentLength
		s.mu.Unlock()
	}

	tmpPath := destPath + ".downloading"
	f, err := os.Create(tmpPath)
	if err != nil {
		return fmt.Errorf("create file: %w", err)
	}
	defer func() {
		f.Close()
		os.Remove(tmpPath)
	}()

	buf := make([]byte, 256*1024)
	var downloaded int64
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		n, readErr := resp.Body.Read(buf)
		if n > 0 {
			if _, writeErr := f.Write(buf[:n]); writeErr != nil {
				return fmt.Errorf("write file: %w", writeErr)
			}
			downloaded += int64(n)
			if entry != nil {
				s.mu.Lock()
				entry.progress.Downloaded = downloaded
				if entry.progress.TotalBytes > 0 {
					entry.progress.Percent = float64(downloaded) / float64(entry.progress.TotalBytes) * 100
				}
				s.mu.Unlock()
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return fmt.Errorf("read body: %w", readErr)
		}
	}
	f.Close()

	if err := os.Rename(tmpPath, destPath); err != nil {
		return fmt.Errorf("rename: %w", err)
	}
	return nil
}

// GetDownloadProgress returns every currently tracked voice download.
func (s *VoiceStore) GetDownloadProgress() []*VoiceDownloadProgress {
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make([]*VoiceDownloadProgress, 0, len(s.downloads))
	for _, entry := range s.downloads {
		p := *entry.progress
		out = append(out, &p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].VoiceID < out[j].VoiceID })
	return out
}

// ListLocalVoices scans voicesDir for complete (.onnx + .onnx.json)
// downloaded voices, matching them back to CuratedVoices() by ID so the
// caller gets structured Locale/Name/Quality fields instead of having to
// re-parse the filename.
func (s *VoiceStore) ListLocalVoices() []LocalVoice {
	knownByID := make(map[string]Voice)
	for _, v := range CuratedVoices() {
		knownByID[v.ID()] = v
	}

	entries, err := os.ReadDir(s.voicesDir)
	if err != nil {
		if !os.IsNotExist(err) {
			logx.Printf("tts.VoiceStore: read voices dir: %v", err)
		}
		return nil
	}

	var out []LocalVoice
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".onnx") {
			continue
		}
		id := strings.TrimSuffix(e.Name(), ".onnx")
		sidecarPath := filepath.Join(s.voicesDir, e.Name()+".json")
		if _, err := os.Stat(sidecarPath); err != nil {
			continue // incomplete download, not a usable voice
		}
		v, known := knownByID[id]
		if !known {
			// A voice downloaded by a filename this build's curated list no
			// longer includes (e.g. after a future curated-list change) is
			// still usable — parse its ID back into locale/name/quality
			// rather than hiding it.
			v = parseVoiceID(id)
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		out = append(out, LocalVoice{
			Voice: v,
			Path:  filepath.Join(s.voicesDir, e.Name()),
			Size:  info.Size(),
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID() < out[j].ID() })
	return out
}

// parseVoiceID recovers a Voice's Locale/Name/Quality from its
// "<locale>-<name>-<quality>" ID (the inverse of Voice.ID), tolerating a
// hyphenated name (e.g. a hypothetical "en_US-north-midwest-medium") by
// treating the first segment as Locale, the last as Quality, and
// everything in between as Name.
func parseVoiceID(id string) Voice {
	parts := strings.Split(id, "-")
	if len(parts) < 3 {
		return Voice{Locale: id}
	}
	return Voice{
		Locale:  parts[0],
		Name:    strings.Join(parts[1:len(parts)-1], "-"),
		Quality: parts[len(parts)-1],
	}
}

// DeleteLocalVoice removes a downloaded voice's .onnx and .onnx.json files.
func (s *VoiceStore) DeleteLocalVoice(id string) error {
	base := filepath.Clean(id)
	if base != id || strings.ContainsAny(id, "/\\") {
		return fmt.Errorf("tts: invalid voice id")
	}
	onnxPath := filepath.Join(s.voicesDir, base+".onnx")
	if err := os.Remove(onnxPath); err != nil && !os.IsNotExist(err) {
		return err
	}
	os.Remove(onnxPath + ".json")
	return nil
}
