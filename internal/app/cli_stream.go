// SPDX-License-Identifier: AGPL-3.0-or-later

package app

import (
	"context"
	"fmt"
	"strings"

	"memo/internal/agentcli"
	"memo/internal/api"
	"memo/internal/logx"
	"memo/internal/provider"
	"memo/internal/sessions"
)

// SetChatCLIProvider/GetChatCLIProvider/SetChatCLIWorkdir/GetChatCLIWorkdir
// are thin App-level wrappers over sessions.Manager's per-chat CLI fields,
// for the webserver bridge (which doesn't call sessions.Manager directly).
func (a *App) SetChatCLIProvider(chatID, cliType string) error {
	sm := a.getSessionManager()
	if sm == nil {
		return fmt.Errorf("sessions not initialized")
	}
	return sm.SetCLIProvider(chatID, cliType)
}

func (a *App) GetChatCLIProvider(chatID string) string {
	sm := a.getSessionManager()
	if sm == nil {
		return ""
	}
	return sm.GetCLIProvider(chatID)
}

func (a *App) SetChatCLIWorkdir(chatID, dir string) error {
	sm := a.getSessionManager()
	if sm == nil {
		return fmt.Errorf("sessions not initialized")
	}
	return sm.SetCLIWorkdir(chatID, dir)
}

func (a *App) GetChatCLIWorkdir(chatID string) string {
	sm := a.getSessionManager()
	if sm == nil {
		return ""
	}
	return sm.GetCLIWorkdir(chatID)
}

func (a *App) SetChatCLIModel(chatID, model string) error {
	sm := a.getSessionManager()
	if sm == nil {
		return fmt.Errorf("sessions not initialized")
	}
	return sm.SetCLIModel(chatID, model)
}

func (a *App) GetChatCLIModel(chatID string) string {
	sm := a.getSessionManager()
	if sm == nil {
		return ""
	}
	return sm.GetCLIModel(chatID)
}

// ListCLIModels returns the model ids cliType's chat can be switched to via
// its per-chat override — the backing data for the top bar's model picker in
// CLI mode. Empty means the CLI has no override list available right now
// (e.g. Codex before its own models_cache.json exists), not an error.
func (a *App) ListCLIModels(cliType string) []string {
	return agentcli.AvailableModels(provider.ProviderType(cliType))
}

// ListCLICommands returns the slash commands cliType offers for chatID's
// working directory — the backing data for the composer's "/" dropdown when
// a chat is in CLI mode. cliType empty (not a CLI chat) yields nothing.
// Returns a concrete slice rather than an interface so the JSON shape is
// pinned by agentcli.Command, and never errors: an absent commands
// directory is an ordinary state, not a failure.
func (a *App) ListCLICommands(cliType, chatID string) []agentcli.Command {
	if strings.TrimSpace(cliType) == "" {
		return nil
	}
	workdir := ""
	if chatID != "" {
		workdir = a.GetChatCLIWorkdir(chatID)
	}
	return agentcli.ListCommands(provider.ProviderType(cliType), workdir)
}

// startCLIJob registers chatID as having an in-flight CLI stream, returning
// false (and doing nothing) if one is already running there. Different
// chats never contend with each other — only two messages racing into the
// *same* chat are rejected, unlike streamMu's app-wide exclusivity.
func (a *App) startCLIJob(chatID string, cancel context.CancelFunc) bool {
	a.cliJobsMu.Lock()
	defer a.cliJobsMu.Unlock()
	if a.cliJobs == nil {
		a.cliJobs = make(map[string]context.CancelFunc)
	}
	if _, running := a.cliJobs[chatID]; running {
		return false
	}
	a.cliJobs[chatID] = cancel
	return true
}

// finishCLIJob clears chatID's in-flight marker. Safe to call even if
// nothing was registered.
func (a *App) finishCLIJob(chatID string) {
	a.cliJobsMu.Lock()
	defer a.cliJobsMu.Unlock()
	delete(a.cliJobs, chatID)
}

// GetRunningCLIChats returns the chat ids with a CLI stream currently in
// flight — polled by the chat sidebar to show a "processing" indicator.
func (a *App) GetRunningCLIChats() []string {
	a.cliJobsMu.Lock()
	defer a.cliJobsMu.Unlock()
	ids := make([]string, 0, len(a.cliJobs))
	for id := range a.cliJobs {
		ids = append(ids, id)
	}
	return ids
}

// IsCLIJobRunning reports whether chatID currently has a CLI stream in
// flight — the backing signal for the chat sidebar's "processing" indicator
// (yapacam.md §2.5).
func (a *App) IsCLIJobRunning(chatID string) bool {
	a.cliJobsMu.Lock()
	defer a.cliJobsMu.Unlock()
	_, ok := a.cliJobs[chatID]
	return ok
}

// CancelCLIJob stops chatID's in-flight CLI stream, if any. Returns false if
// nothing was running. The stream goroutine's own recvChunk/ctx-cancelled
// path is what actually sends the terminal "⏹️ Cevap durduruldu." chunk and
// calls finishCLIJob — this only signals it to stop.
func (a *App) CancelCLIJob(chatID string) bool {
	a.cliJobsMu.Lock()
	cancel, ok := a.cliJobs[chatID]
	a.cliJobsMu.Unlock()
	if !ok {
		return false
	}
	cancel()
	return true
}

// SendCLIMessageStream sends userMsg to chatID's own CLI-backed provider
// (Session.CLIProvider) and streams the reply back.
//
// Deliberately independent of SendMessageStreamTo/sendMessageStreamInnerTo
// and the app-wide a.streamMu they serialize through — a CLI task can run
// for a long time with no fixed timeout (yapacam.md §7) and must not block
// every other chat in the app for that whole duration. Exclusivity here is
// per-chat only, via startCLIJob/finishCLIJob.
//
// Also deliberately bypasses routeStream/internal/agent entirely: Memo's own
// tool-execution pipeline must never run alongside a CLI provider, which
// does its own file/shell work internally and opaquely (yapacam.md §5).
//
// The passed-in ctx (normally the HTTP request's) is intentionally NOT what
// controls the job's lifetime — kept only for signature symmetry with
// SendMessageStreamTo. The subprocess must survive the user switching chats
// or closing the app window (the HTTP/SSE connection ending), and only stop
// on an explicit CancelCLIJob or a real backend shutdown (yapacam.md
// §2.5/§7), so the job is tied to a.lifecycleCtx instead. Accepted
// limitation for now: if outCh's buffer (128 chunks) ever fills while no one
// is reading it (tab closed mid-stream on an unusually long/chatty reply),
// this goroutine blocks in trySend until the job is cancelled or the app
// shuts down — the full "reconnect to a live background job" UI (sidebar
// indicator, yapacam.md §2.5) is what actually closes this gap, not yet
// built.
func (a *App) SendCLIMessageStream(ctx context.Context, chatID, userMsg string) <-chan api.StreamChunk {
	sm := a.getSessionManager()
	if sm == nil || !sm.SessionExists(chatID) {
		return errStreamChunk(fmt.Sprintf("sohbet bulunamadı: %s", chatID))
	}

	cliType := sm.GetCLIProvider(chatID)
	if cliType == "" {
		return errStreamChunk("bu sohbette bir CLI sağlayıcı seçili değil")
	}

	jobBase := a.lifecycleCtx
	if jobBase == nil {
		// Only happens outside a real Startup() (unit tests constructing a
		// bare *App{}) — production always has a lifecycleCtx by the time
		// any chat can be sent.
		jobBase = context.Background()
	}
	cliCtx, cancel := context.WithCancel(jobBase)
	if !a.startCLIJob(chatID, cancel) {
		cancel()
		return errStreamChunk("⏳ Bu sohbette zaten bir CLI görevi çalışıyor.")
	}

	// Saved synchronously, before the goroutine even starts — the normal
	// (non-CLI) path saves the user's turn in sendMessageStreamCore before
	// routeStream ever runs, and this must match: outCh is returned to the
	// caller immediately, so if this were deferred into the goroutine a fast
	// cancel/dismiss could race ahead of it, and a chat re-fetched from disk
	// (leaving and returning to it) must always show the message the user
	// actually sent, not just the reply.
	sm.AddMessageToSession(chatID, "user", userMsg, "", "")

	outCh := make(chan api.StreamChunk, 128)
	go func() {
		defer close(outCh)
		defer a.finishCLIJob(chatID)
		defer cancel()
		defer recoverStreamPanic(cliCtx, outCh, "SendCLIMessageStream")

		p, err := a.resolveCLIProvider(cliType)
		if err != nil {
			sm.AddMessageToSession(chatID, "assistant", "⚠️ "+err.Error(), "", "")
			trySend(cliCtx, outCh, api.StreamChunk{Error: "⚠️ " + err.Error(), Done: true})
			return
		}

		req := provider.ChatRequest{
			Messages:        []provider.Message{provider.TextMessage("user", userMsg)},
			ResumeSessionID: sm.GetCLISessionID(chatID, cliType),
			WorkDir:         sm.GetCLIWorkdir(chatID),
			Model:           sm.GetCLIModel(chatID),
		}

		ch, err := p.ChatCompletionStream(cliCtx, req)
		if err != nil {
			errMsg := "⚠️ " + err.Error()
			sm.AddMessageToSession(chatID, "assistant", errMsg, "", "")
			trySend(cliCtx, outCh, api.StreamChunk{Error: errMsg, Done: true})
			return
		}

		var fullReply strings.Builder
		for {
			chunk, ok, ctxDone := recvChunk(cliCtx, ch)
			if ctxDone {
				a.recordStreamError(userMsg, "⏹️ Cevap durduruldu.", chatID)
				trySend(cliCtx, outCh, api.StreamChunk{Error: "⏹️ Cevap durduruldu.", Done: true})
				return
			}
			if !ok {
				break
			}

			if chunk.CLISessionID != "" {
				if err := sm.SetCLISessionID(chatID, cliType, chunk.CLISessionID); err != nil {
					logx.Printf("WARN: SetCLISessionID(%s, %s): %v", chatID, cliType, err)
				}
			}

			if chunk.Error != "" {
				errMsg := "⚠️ " + chunk.Error
				a.recordStreamError(userMsg, errMsg, chatID)
				trySend(cliCtx, outCh, api.StreamChunk{Error: errMsg, Done: true})
				return
			}

			if chunk.Content != "" {
				fullReply.WriteString(chunk.Content)
				trySend(cliCtx, outCh, api.StreamChunk{Content: chunk.Content})
			}

			if chunk.Done {
				sm.AddMessageToSession(chatID, "assistant", fullReply.String(), "", "")
				a.generateCLIChatTitleIfNew(chatID, sm)
				trySend(cliCtx, outCh, api.StreamChunk{Done: true, FinishReason: chunk.FinishReason})
				return
			}
		}

		// Stream closed without an explicit Done chunk — still must send a
		// terminal chunk (AGENTS.md streaming gotcha: every branch that ends
		// a stream must, or the frontend waits forever).
		if fullReply.Len() > 0 {
			sm.AddMessageToSession(chatID, "assistant", fullReply.String(), "", "")
			a.generateCLIChatTitleIfNew(chatID, sm)
		}
		trySend(cliCtx, outCh, api.StreamChunk{Done: true, FinishReason: "stop"})
	}()

	return outCh
}

// generateCLIChatTitleIfNew mirrors finishStream's own title-generation
// trigger (internal/app/llm.go) — SendCLIMessageStream doesn't go through
// finishStream at all (see its own doc comment), so this is duplicated
// rather than shared, matching the same "exactly 2 messages" condition.
func (a *App) generateCLIChatTitleIfNew(chatID string, sm *sessions.Manager) {
	if len(sm.GetActiveMessagesForSession(chatID)) == 2 {
		goRecover("generateChatTitleForSession", func() { a.generateChatTitleForSession(chatID) })
	}
}

// resolveCLIProvider builds a provider.Provider for the given CLI provider
// type from its configured ProviderConfig (added via Settings > CLI
// Bağlantıları, yapacam.md §8).
func (a *App) resolveCLIProvider(cliType string) (provider.Provider, error) {
	a.providerMu.RLock()
	cfgMgr := a.providerCfgMgr
	a.providerMu.RUnlock()
	if cfgMgr == nil {
		return nil, fmt.Errorf("provider system not initialized")
	}

	var cfg *provider.ProviderConfig
	for _, p := range cfgMgr.GetEnabled() {
		if string(p.Type) == cliType {
			pc := p
			cfg = &pc
			break
		}
	}
	if cfg == nil {
		return nil, fmt.Errorf("%s yapılandırılmamış. Ayarlar > CLI Bağlantıları'ndan ekleyin.", cliType)
	}
	return provider.NewProvider(*cfg)
}

// errStreamChunk returns a single-chunk, already-closed error stream — the
// same shape SendMessageStreamTo uses for its own early-return error cases.
func errStreamChunk(msg string) <-chan api.StreamChunk {
	ch := make(chan api.StreamChunk, 1)
	ch <- api.StreamChunk{Error: msg, Done: true}
	close(ch)
	return ch
}
