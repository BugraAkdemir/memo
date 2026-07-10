package app

import (
	"context"
	"encoding/base64"
	"fmt"
	"memo/internal/logx"
	"os"
	"path/filepath"
	"strings"
	"time"

	"memo/internal/config"
	"memo/internal/identity"
)

// GetConfig returns the full application configuration.
func (a *App) GetConfig() *config.AppConfig { return a.cfg }

// GetAvailableStyles returns valid identity style options.
func (a *App) GetAvailableStyles() []string { return identity.AvailableStyles() }

// UpdateIdentity updates user/assistant name and chat style.
func (a *App) UpdateIdentity(userName, assistantName, style string) error {
	a.identity.Update(userName, assistantName, style, a.cfg.Identity.SystemRole)
	a.cfg.Identity.UserName = userName
	a.cfg.Identity.AssistantName = assistantName
	a.cfg.Identity.Style = style
	return config.Save(a.cfg)
}

// UpdateMoodConfig duygu motorunu günceller ve config.yaml'a kaydeder.
func (a *App) UpdateMoodConfig(enabled bool) error {
	a.cfg.Mood.Enabled = enabled
	if a.mood != nil {
		a.mood.SetEnabled(enabled)
	}
	return config.Save(a.cfg)
}

// UpdateSelfInterestConfig öz-çıkar protokolünü açar/kapatır ve config.yaml'a kaydeder.
// SelfInterest kapatılırsa SystemManagement da otomatik kapatılır ve bypass sıfırlanır.
func (a *App) UpdateSelfInterestConfig(enabled bool) error {
	a.cfg.Mood.SelfInterest = enabled
	if a.mood != nil {
		a.mood.SetSelfInterest(enabled)
	}
	if !enabled {
		a.cfg.Mood.SystemManagement = false
		if a.mood != nil {
			a.mood.SetSystemManagement(false)
		}
		if a.agentExecutor != nil {
			a.agentExecutor.SetBypassPermissions(false)
		}
	}
	return config.Save(a.cfg)
}

// GetSelfInterestEnabled öz-çıkar protokolünün durumunu döner.
func (a *App) GetSelfInterestEnabled() bool {
	if a.mood == nil {
		return false
	}
	return a.mood.SelfInterestEnabled()
}

// UpdateWebSearchConfig web arama özelliğini günceller ve config.yaml'a kaydeder.
func (a *App) UpdateWebSearchConfig(enabled bool) error {
	a.cfgMu.Lock()
	a.cfg.WebSearch.Enabled = enabled
	a.cfgMu.Unlock()
	return config.Save(a.cfg)
}

// GetWebSearchEnabled web arama özelliğinin durumunu döner.
func (a *App) GetWebSearchEnabled() bool {
	a.cfgMu.RLock()
	defer a.cfgMu.RUnlock()
	return a.cfg.WebSearch.Enabled
}

// UpdateSystemManagementConfig sistem yönetimi modunu günceller.
func (a *App) UpdateSystemManagementConfig(enabled bool) error {
	a.cfg.Mood.SystemManagement = enabled
	if a.mood != nil {
		a.mood.SetSystemManagement(enabled)
	}
	// Agent izin ekranı bypass — sistem yönetimi açıkken hiç izin sorulmaz
	if a.agentExecutor != nil {
		a.agentExecutor.SetBypassPermissions(enabled)
	}
	return config.Save(a.cfg)
}

// GetSystemManagementEnabled sistem yönetimi modunun durumunu döner.
func (a *App) GetSystemManagementEnabled() bool {
	if a.mood == nil {
		return false
	}
	return a.mood.SystemManagementEnabled()
}

// GetMoodEnabled mood motorunun açık/kapalı durumunu döner (UI için).
func (a *App) GetMoodEnabled() bool {
	if a.mood == nil {
		return false
	}
	return a.mood.Enabled()
}

// GetMoodScore mevcut duygu skorunu döner (UI için).
func (a *App) GetMoodScore() float64 {
	if a.mood == nil {
		return 0.0
	}
	return a.mood.Score()
}

// CheckConnection tests connectivity to the configured LLM endpoint.
func (a *App) CheckConnection() ConnectionStatus {
	tctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	a.clientMu.RLock()
	checkClient := a.client
	a.clientMu.RUnlock()
	models, err := checkClient.CheckConnection(tctx)
	if err != nil {
		return ConnectionStatus{Connected: false, Error: err.Error()}
	}
	var names []string
	for _, m := range models {
		names = append(names, m.ID)
	}
	return ConnectionStatus{Connected: true, Models: names}
}

// GetSystemPrompt returns the current system prompt.
func (a *App) GetSystemPrompt() string {
	return a.cfg.Identity.SystemRole
}

// GetIncognitoPrompt returns the incognito system prompt.
func (a *App) GetIncognitoPrompt() string {
	return a.cfg.Identity.IncognitoPrompt
}

// SetIncognitoPrompt updates the incognito mode system prompt.
func (a *App) SetIncognitoPrompt(prompt string) error {
	a.cfg.Identity.IncognitoPrompt = prompt
	return config.Save(a.cfg)
}

// SetSystemPrompt replaces the system prompt.
func (a *App) SetSystemPrompt(prompt string) error {
	a.identity.Update("", "", "", prompt)
	a.cfg.Identity.SystemRole = prompt
	logx.Printf("System prompt updated (%d chars)", len(prompt))
	return config.Save(a.cfg)
}

// ResetSystemPrompt restores the default system prompt.
func (a *App) ResetSystemPrompt() error {
	nameSection := ""
	if a.cfg.Identity.UserName != "" {
		nameSection = fmt.Sprintf("The user's name is %s. ", a.cfg.Identity.UserName)
	}
	defaultPrompt := fmt.Sprintf(`%sYou are %s, a highly capable, privacy-first AI assistant running entirely locally on the user's device.

CORE DIRECTIVES:
1. Identity: You are always %s, regardless of the underlying LLM. Act as a smart, reliable, and direct partner.
2. Anti-Hallucination: Never invent, guess, or fabricate information. If you are unsure or do not know the answer, explicitly state that you do not know.
3. Conciseness & Structure: Keep your answers clear, well-structured, and strictly to the point. Avoid long, rambling introductions or unnecessary filler words.
4. Seamless Memory: You have access to the user's personal context. Use this information naturally to inform your answers. STRICTLY FORBIDDEN: Do not use phrases like "I remember," "As we discussed," "Based on your data," or "I recall." Simply present the information as shared context.
5. Language Mirroring: Always respond in the exact language the user communicates in (e.g., if the user asks in Turkish, your entire response must be in Turkish).`, nameSection, a.cfg.Identity.AssistantName, a.cfg.Identity.AssistantName)

	a.identity.Update("", "", "", defaultPrompt)
	a.cfg.Identity.SystemRole = defaultPrompt
	logx.Info("System prompt reset to default")
	return config.Save(a.cfg)
}

// GetMinimalMode reports whether identity/persona/mood/web-search prompt
// injection is disabled — only memory context (if separately enabled)
// still reaches the model. This reads the persisted config copy (status
// reporting / API); the hot streaming path checks a.identity.GetMinimalMode()
// directly instead — see buildMessages.
func (a *App) GetMinimalMode() bool {
	a.cfgMu.RLock()
	defer a.cfgMu.RUnlock()
	return a.cfg.Identity.MinimalMode
}

// SetMinimalMode toggles minimal mode.
func (a *App) SetMinimalMode(enabled bool) error {
	a.cfgMu.Lock()
	a.cfg.Identity.MinimalMode = enabled
	a.cfgMu.Unlock()
	a.identity.SetMinimalMode(enabled)
	logx.Printf("Minimal mode set to %v", enabled)
	return config.Save(a.cfg)
}

// GetImageBase64 reads an image file from within the data directory and returns it as base64.
func (a *App) GetImageBase64(path string) string {
	dataDir := filepath.Dir(a.cfg.Memory.PersistDir)
	absDataDir, err := filepath.Abs(dataDir)
	if err != nil {
		return ""
	}

	absPath, err := filepath.Abs(path)
	if err != nil {
		return ""
	}

	realPath, err := filepath.EvalSymlinks(absPath)
	if err != nil {
		return ""
	}

	if !strings.HasPrefix(realPath, absDataDir) {
		logx.Printf("WARNING: Blocked attempt to read file outside data dir: %s", path)
		return ""
	}

	imgData, err := os.ReadFile(realPath)
	if err != nil {
		return ""
	}
	mime := detectMime(path, imgData)
	return "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(imgData)
}

// ─── Web Bridge (interface adapters for webserver) ───────────────

// WebListChats wraps ListChats for the webserver bridge.
func (a *App) WebListChats() interface{} { return a.ListChats() }

// WebGetActiveMessages wraps GetActiveMessages for the webserver bridge.
func (a *App) WebGetActiveMessages() interface{} { return a.GetActiveMessages() }

// WebCheckConnection wraps CheckConnection for the webserver bridge.
func (a *App) WebCheckConnection() interface{} { return a.CheckConnection() }

// findPath resolves a relative path, first against the working directory,
// then against the binary location.
func (a *App) findPath(relative string) string {
	if _, err := os.Stat(relative); err == nil {
		return relative
	}
	exePath, err := os.Executable()
	if err != nil {
		exePath = os.Args[0]
	}
	full := filepath.Join(filepath.Dir(exePath), relative)
	if _, err := os.Stat(full); err == nil {
		return full
	}
	return ""
}
