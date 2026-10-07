package app

import (
	"context"
	"encoding/base64"
	"fmt"
	"memo/internal/agent/tools"
	"memo/internal/browserengine"
	"memo/internal/logx"
	"os"
	"path/filepath"
	"strings"
	"time"

	"memo/internal/config"
	"memo/internal/identity"
	"memo/internal/llama"
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

// GetBrowserKeepAlive tarayıcı motorunun sürekli açık mı yoksa her
// kullanımdan sonra kapanıyor mu olduğunu döner.
func (a *App) GetBrowserKeepAlive() bool {
	a.cfgMu.RLock()
	defer a.cfgMu.RUnlock()
	return a.cfg.Browser.KeepAlive
}

// SetBrowserKeepAlive tarayıcı motorunun modunu değiştirir ve config.yaml'a
// kaydeder. Değişiklik anında etkili olur — bkz. Manager.SetKeepAlive.
func (a *App) SetBrowserKeepAlive(keepAlive bool) error {
	a.cfgMu.Lock()
	a.cfg.Browser.KeepAlive = keepAlive
	cfg := a.cfg
	a.cfgMu.Unlock()
	if a.browserMgr != nil {
		if err := a.browserMgr.SetKeepAlive(keepAlive); err != nil {
			logx.Printf("browser engine: SetKeepAlive(%v): %v", keepAlive, err)
		}
	}
	return config.Save(cfg)
}

// GetBrowserInstalled tarayıcı motorunun (Chromium ailesi) sistemde
// kurulu/kullanılabilir olup olmadığını döner — hiçbir şey indirmez ya da
// başlatmaz.
func (a *App) GetBrowserInstalled(ctx context.Context) bool {
	if a.browserMgr == nil {
		return false
	}
	return a.browserMgr.IsInstalled(ctx)
}

// InstallBrowser tarayıcı motoru indirmeyi arka planda başlatır ve hemen
// döner — ilerleme için GetBrowserInstallProgress'i poll'la.
func (a *App) InstallBrowser(ctx context.Context) error {
	if a.browserMgr == nil {
		return fmt.Errorf("browser engine not initialized")
	}
	a.browserMgr.StartInstall(ctx)
	return nil
}

// GetBrowserInstallProgress mevcut (ya da en son biten) kurulum denemesinin
// anlık durumunu döner.
func (a *App) GetBrowserInstallProgress() browserengine.InstallProgress {
	if a.browserMgr == nil {
		return browserengine.InstallProgress{}
	}
	return a.browserMgr.InstallProgress()
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

// GetCodeSubModePrompt returns subMode's ("plan"/"auto"/"build") Settings
// override, or "" if none is set — codeSubModeDirective (agent_chat_context.go)
// falls back to the built-in const for that sub-mode in that case, the same
// empty-means-default shape GetSystemPrompt's siblings use.
func (a *App) GetCodeSubModePrompt(subMode string) string {
	a.cfgMu.RLock()
	defer a.cfgMu.RUnlock()
	switch subMode {
	case "plan":
		return a.cfg.AgentMode.CodePlanPrompt
	case "build":
		return a.cfg.AgentMode.CodeBuildPrompt
	default: // "auto" and ""
		return a.cfg.AgentMode.CodeAutoPrompt
	}
}

// SetCodeSubModePrompt overrides subMode's Code Mode system prompt.
func (a *App) SetCodeSubModePrompt(subMode, prompt string) error {
	a.cfgMu.Lock()
	switch subMode {
	case "plan":
		a.cfg.AgentMode.CodePlanPrompt = prompt
	case "auto", "":
		a.cfg.AgentMode.CodeAutoPrompt = prompt
	case "build":
		a.cfg.AgentMode.CodeBuildPrompt = prompt
	default:
		a.cfgMu.Unlock()
		return fmt.Errorf("unknown code sub-mode: %q", subMode)
	}
	cfg := a.cfg
	a.cfgMu.Unlock()
	logx.Printf("Code Mode %s prompt updated (%d chars)", subMode, len(prompt))
	return config.Save(cfg)
}

// ResetCodeSubModePrompt clears subMode's override, falling back to its
// built-in default (codeSubModeDirective).
func (a *App) ResetCodeSubModePrompt(subMode string) error {
	if err := a.SetCodeSubModePrompt(subMode, ""); err != nil {
		return err
	}
	logx.Printf("Code Mode %s prompt reset to default", subMode)
	return nil
}

// GetUILanguage returns the GUI's last-known display language ("tr"/"en",
// or "" if never set). The backend never picks this itself — it's purely
// what the Flutter GUI last wrote via SetUILanguage.
func (a *App) GetUILanguage() string {
	a.cfgMu.RLock()
	defer a.cfgMu.RUnlock()
	return a.cfg.Identity.UILanguage
}

// SetUILanguage persists the GUI's current display language so other
// clients with no local preference store of their own (the terminal REPL)
// can follow it.
func (a *App) SetUILanguage(lang string) error {
	a.cfgMu.Lock()
	a.cfg.Identity.UILanguage = lang
	a.cfgMu.Unlock()
	// Tool result strings live in internal/agent/tools, which has no App
	// access — keep their process-wide language in sync (see tools/l10n.go).
	tools.SetUILanguage(lang)
	// Same story for the llama.cpp installer's progress/error strings.
	llama.SetUILanguage(lang)
	return config.Save(a.cfg)
}

// t picks the user-facing message variant for the active UI language:
// Turkish when Identity.UILanguage is exactly "tr", English otherwise —
// including unset, matching waLang's convention (the GUI's own default has
// been English since 2026-08-13, and "" means the toggle was never
// touched). This closes the long-standing seam where backend-generated
// strings that reach the chat window or API responses ("⏹️ Cevap
// durduruldu.", model-load errors, memory confirmations) were Turkish
// regardless of what language the rest of the UI spoke.
//
// Scope: internal/app's own chat-SSE/response strings. The dedicated
// surfaces already have their own tables driven by the same setting —
// replcli/l10n.go (CLI, TR-default carve-out) and whatsapp_l10n.go /
// telegram_l10n.go — and strings aimed at the *model* (Turkish system
// prompts, tool descriptions) are deliberately not routed through here:
// changing those changes model behavior, not UI language.
//
// Nil-cfg guard: tests construct bare &App{} values that now flow through
// converted strings; production Apps always have cfg, and a nil cfg has no
// persisted preference — the English default is right there too.
//
// fmt verbs survive intact: t returns the template; callers Sprintf it.
func (a *App) t(tr, en string) string {
	if a.cfg != nil && a.GetUILanguage() == "tr" {
		return tr
	}
	return en
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

// GetMinimalModeOverrides returns the current granular Minimal Mode
// overrides as plain primitives (see identity.Identity.GetMinimalModeOverrides's
// doc comment — each only has any effect while Minimal Mode itself is on).
// Plain bools rather than a struct, same as the rest of this bridge (e.g.
// ImportMemoryFromText), so internal/webserver never needs to import
// internal/app types directly.
func (a *App) GetMinimalModeOverrides() (keepPersona, keepCapabilities, keepPassive, keepProactive bool) {
	a.cfgMu.RLock()
	defer a.cfgMu.RUnlock()
	return a.cfg.Identity.MinimalModeKeepPersona, a.cfg.Identity.MinimalModeKeepCapabilities,
		a.cfg.Identity.MinimalModeKeepPassive, a.cfg.Identity.MinimalModeKeepProactive
}

// SetMinimalModeOverrides updates the granular Minimal Mode overrides.
func (a *App) SetMinimalModeOverrides(keepPersona, keepCapabilities, keepPassive, keepProactive bool) error {
	a.cfgMu.Lock()
	a.cfg.Identity.MinimalModeKeepPersona = keepPersona
	a.cfg.Identity.MinimalModeKeepCapabilities = keepCapabilities
	a.cfg.Identity.MinimalModeKeepPassive = keepPassive
	a.cfg.Identity.MinimalModeKeepProactive = keepProactive
	a.cfgMu.Unlock()
	a.identity.SetMinimalModeOverrides(keepPersona, keepCapabilities, keepPassive, keepProactive)
	logx.Printf("Minimal mode overrides set: persona=%v capabilities=%v passive=%v proactive=%v",
		keepPersona, keepCapabilities, keepPassive, keepProactive)
	return config.Save(a.cfg)
}

// chatImageDirs are the only directories GetImageBase64 serves from: where a
// chat message's image can legitimately live. Resolved from config.DataDir
// (never a hardcoded "data/"), so they follow MEMO_DATA_DIR and the Windows
// %ProgramData% layout.
func chatImageDirs() []string {
	return []string{
		config.DataPath("images"),           // images the user sent (persistChatImage)
		config.DataPath(generatedImagesSub), // images a model generated (imagegen.go)
		config.DataPath("avatars"),
		config.DataPath("attachments"),
	}
}

// GetImageBase64 reads a chat image and returns it as a data: URI — the way a
// client that is not on the backend's machine (web, the mobile app, remote
// access) displays one.
//
// Only files inside chatImageDirs are served. The check is on the resolved
// real path (symlinks followed on both sides) and is separator-safe: the old
// check was a bare string prefix against the data dir, which also admitted a
// sibling like "<data>-backup/…", and it compared against an unresolved data
// dir, so a data dir reached through a symlink blocked every legitimate file.
// A relative path is taken as-is (relative to the working directory), which
// keeps the historical "data/images/…" form working.
func (a *App) GetImageBase64(path string) string {
	realPath, err := filepath.EvalSymlinks(path)
	if err != nil {
		return ""
	}
	realPath, err = filepath.Abs(realPath)
	if err != nil {
		return ""
	}
	allowed := false
	for _, dir := range chatImageDirs() {
		root, err := filepath.EvalSymlinks(dir)
		if err != nil {
			continue // the directory doesn't exist yet — nothing in it to serve
		}
		if root, err = filepath.Abs(root); err != nil {
			continue
		}
		if strings.HasPrefix(realPath, root+string(filepath.Separator)) {
			allowed = true
			break
		}
	}
	if !allowed {
		logx.Printf("WARNING: Blocked attempt to read a file outside the chat image directories: %s", path)
		return ""
	}

	imgData, err := readImageFile(realPath)
	if err != nil {
		return ""
	}
	mime := detectMime(path, imgData)
	return "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(imgData)
}

// persistChatImage keeps a copy of an image the user sent under
// DataPath("images") and returns the copy's path, which is what the chat
// message records. The path a send arrives with is often not durable: a web
// or mobile upload is a temp file the upload handler deletes as soon as the
// turn is set up, so the stored message pointed at a file that no longer
// existed and the image could never be shown again, on any client. On
// failure the original path is returned so the turn itself still goes ahead.
func persistChatImage(src string, data []byte) string {
	dir := config.DataPath("images")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		logx.Printf("WARN: persist chat image: %v", err)
		return src
	}
	ext := strings.ToLower(filepath.Ext(src))
	if ext == "" || len(ext) > 6 {
		ext = ".img"
	}
	dst := filepath.Join(dir, fmt.Sprintf("memo-%d%s", time.Now().UnixNano(), ext))
	if err := writeImageFile(dst, data); err != nil {
		logx.Printf("WARN: persist chat image: %v", err)
		return src
	}
	return dst
}

// ─── Web Bridge (interface adapters for webserver) ───────────────

// WebListChats wraps ListChats for the webserver bridge.
func (a *App) WebListChats() interface{} { return a.ListChats() }

// WebGetActiveMessages wraps GetActiveMessages for the webserver bridge.
func (a *App) WebGetActiveMessages() interface{} { return a.GetActiveMessages() }

// WebCheckConnection wraps CheckConnection for the webserver bridge.
func (a *App) WebCheckConnection() interface{} { return a.CheckConnection() }
