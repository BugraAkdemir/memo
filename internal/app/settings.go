package app

import (
	"context"
	"encoding/base64"
	"fmt"
	"log"
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
	log.Printf("System prompt updated (%d chars)", len(prompt))
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
	log.Println("System prompt reset to default")
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
		log.Printf("WARNING: Blocked attempt to read file outside data dir: %s", path)
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
