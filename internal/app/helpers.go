package app

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"memo/internal/api"
	"memo/internal/memory"
	"memo/internal/truncate"
)

func (a *App) buildMessages(ctx context.Context, userMsg string, extraImageB64 []string) []api.Message {
	var memories []memory.MemoryResult
	if a.cfg.Memory.MemoryEnabled {
		memories = a.retrieveMemory(ctx, userMsg)
	}
	systemPrompt := a.identity.BuildSystemPrompt(memories)
	if a.mood != nil {
		systemPrompt += a.mood.BuildDirective()
		systemPrompt += a.mood.BuildSelfInterestDirective()
	}

	var tokenBudget int
	if a.llamaServer.IsRunning() {
		maxLocal := a.cfg.Llama.CtxSize
		if maxLocal <= 0 {
			maxLocal = 4096
		}
		if a.cfg.Llama.MaxContextTokens > 0 && a.cfg.Llama.MaxContextTokens < maxLocal {
			tokenBudget = a.cfg.Llama.MaxContextTokens
		} else {
			tokenBudget = maxLocal
		}
	} else {
		if a.cfg.Llama.MaxContextTokens > 0 {
			tokenBudget = a.cfg.Llama.MaxContextTokens
		} else {
			a.providerMu.RLock()
			switch a.activeProvider {
			case "gemini":
				tokenBudget = 1024 * 1024
			case "claude":
				tokenBudget = 200 * 1024
			case "openai", "grok", "groq", "openrouter", "ollama":
				tokenBudget = 128 * 1024
			default:
				tokenBudget = 128 * 1024
			}
			a.providerMu.RUnlock()
		}
	}

	systemTokens := truncate.EstimateTokens(systemPrompt)
	userTokens := truncate.EstimateTokens(userMsg)
	historyBudget := tokenBudget - systemTokens - userTokens
	if historyBudget < 512 {
		historyBudget = 512
	}

	history := a.getSessionHistoryTokenAware(historyBudget)
	history = append([]api.Message{}, history...)
	var msgs []api.Message

	if a.llamaServer.IsRunning() {
		if len(history) == 0 {
			combinedMsg := systemPrompt + "\n\n" + userMsg
			if len(extraImageB64) > 0 {
				msgs = append(msgs, api.NewMultimodalMessage("user", combinedMsg, extraImageB64...))
			} else {
				msgs = append(msgs, api.NewTextMessage("user", combinedMsg))
			}
		} else {
			injected := false
			for i, h := range history {
				if !injected && h.Role == "user" {
					content := systemPrompt + "\n\n" + h.GetTextContent()
					history[i] = api.NewTextMessage("user", content)
					injected = true
				}
			}
			msgs = append(msgs, history...)
			if len(extraImageB64) > 0 {
				msgs = append(msgs, api.NewMultimodalMessage("user", userMsg, extraImageB64...))
			} else {
				msgs = append(msgs, api.NewTextMessage("user", userMsg))
			}
		}
	} else {
		msgs = append(msgs, api.NewTextMessage("system", systemPrompt))
		msgs = append(msgs, history...)
		if len(extraImageB64) > 0 {
			msgs = append(msgs, api.NewMultimodalMessage("user", userMsg, extraImageB64...))
		} else {
			msgs = append(msgs, api.NewTextMessage("user", userMsg))
		}
	}

	log.Printf("CONTEXT: budget=%d system=%d user=%d history=%d history_msgs=%d total_msgs=%d",
		tokenBudget, systemTokens, userTokens, historyBudget, len(history), len(msgs))
	return msgs
}

// buildConversationContext extracts conversation history from messages for orchestra chief.
func buildConversationContext(messages []api.Message, userPrompt string) string {
	var sb strings.Builder

	var prevMsgs []api.Message
	for i := len(messages) - 2; i >= 0; i-- {
		if messages[i].Role == "system" {
			continue
		}
		prevMsgs = append(prevMsgs, messages[i])
	}

	for i := len(prevMsgs) - 1; i >= 0; i-- {
		msg := prevMsgs[i]
		roleLabel := "Kullanıcı"
		if msg.Role == "assistant" {
			roleLabel = "Asistan"
		}
		if text, ok := msg.Content.(string); ok {
			cleanText := stripOrchestraLines(text)
			sb.WriteString(fmt.Sprintf("%s: %s\n", roleLabel, cleanText))
		}
	}

	if sb.Len() > 0 {
		ctx := fmt.Sprintf("Önceki konuşma:\n%s\n---\nYeni mesaj:\nKullanıcı: %s", sb.String(), userPrompt)
		return ctx
	}

	return fmt.Sprintf("Kullanıcı: %s", userPrompt)
}

var orchestraPrefixes = []string{"🎵", "🧙", "🧠", "✅", "❌", "📝"}

// stripOrchestraLines removes lines that start with orchestra debug prefixes.
func stripOrchestraLines(text string) string {
	lines := strings.Split(text, "\n")
	var filtered []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		skip := false
		for _, prefix := range orchestraPrefixes {
			if strings.HasPrefix(trimmed, prefix) {
				skip = true
				break
			}
		}
		if !skip && strings.Contains(trimmed, "**: ") {
			for _, role := range []string{"planner", "frontend", "backend", "bug_fixer", "reviewer", "security", "devops", "general"} {
				if strings.HasPrefix(trimmed, "**"+role+"**:") {
					skip = true
					break
				}
			}
		}
		if !skip && strings.HasPrefix(trimmed, "Sistem talimatları:") {
			skip = true
		}
		if !skip {
			filtered = append(filtered, line)
		}
	}
	return strings.Join(filtered, "\n")
}

func (a *App) getSessionHistory() []api.Message {
	return a.getSessionHistoryTokenAware(0)
}

func (a *App) getSessionHistoryTokenAware(tokenBudget int) []api.Message {
	sm := a.getSessionManager()
	if sm == nil {
		return nil
	}

	var history []map[string]string
	if tokenBudget > 0 {
		history = sm.GetHistoryForAPITokenAware(tokenBudget)
	} else {
		history = sm.GetHistoryForAPI(a.cfg.Llama.MaxHistory)
	}

	var msgs []api.Message
	for _, h := range history {
		msgs = append(msgs, api.NewTextMessage(h["role"], h["content"]))
	}
	return msgs
}

func detectMime(path string, data []byte) string {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".png":
		return "image/png"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	case ".bmp":
		return "image/bmp"
	}
	mime := http.DetectContentType(data)
	if mime == "application/octet-stream" {
		return "image/jpeg"
	}
	return mime
}

// downloadFile downloads a GGUF model from HuggingFace synchronously.
func (a *App) downloadFile(repoID, filename, destPath string) error {
	downloadURL := fmt.Sprintf("https://huggingface.co/%s/resolve/main/%s", repoID, filename)

	req, err := http.NewRequestWithContext(a.lifecycleCtx, http.MethodGet, downloadURL, nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	dlClient := &http.Client{Timeout: 0}
	resp, err := dlClient.Do(req)
	if err != nil {
		return fmt.Errorf("download request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("download status %d: %s", resp.StatusCode, string(body))
	}

	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		return fmt.Errorf("create dir: %w", err)
	}

	tmpPath := destPath + ".downloading"
	f, err := os.Create(tmpPath)
	if err != nil {
		return fmt.Errorf("create file: %w", err)
	}
	defer f.Close()

	written, err := io.Copy(f, resp.Body)
	if err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("download write: %w", err)
	}
	f.Close()

	if err := os.Rename(tmpPath, destPath); err != nil {
		if copyErr := copyFile(tmpPath, destPath); copyErr != nil {
			os.Remove(tmpPath)
			return fmt.Errorf("rename and copy fallback both failed: rename: %w, copy: %v", err, copyErr)
		}
		os.Remove(tmpPath)
	}

	log.Printf("Downloaded %s (%d bytes)", destPath, written)
	return nil
}

// copyFile copies a file from src to dst (cross-device safe).
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}

func truncateLog(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
