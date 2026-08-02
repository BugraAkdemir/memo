// SPDX-License-Identifier: AGPL-3.0-or-later

package app

import (
	"context"
	"fmt"
	"strings"

	"memo/internal/api"
	"memo/internal/logx"
	"memo/internal/provider"
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
				trySend(cliCtx, outCh, api.StreamChunk{Done: true, FinishReason: chunk.FinishReason})
				return
			}
		}

		// Stream closed without an explicit Done chunk — still must send a
		// terminal chunk (AGENTS.md streaming gotcha: every branch that ends
		// a stream must, or the frontend waits forever).
		if fullReply.Len() > 0 {
			sm.AddMessageToSession(chatID, "assistant", fullReply.String(), "", "")
		}
		trySend(cliCtx, outCh, api.StreamChunk{Done: true, FinishReason: "stop"})
	}()

	return outCh
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
