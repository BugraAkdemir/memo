package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"memo/internal/config"
	"memo/internal/replcli"
)

// ensureBackendRunning attaches to a backend already listening on port, or
// spawns a detached one and waits for it to come up — the same dance
// runPrintMode, runChatListMode and runChatMemoryMode all need before they
// can talk to the REST API. Extracted once these scripting flags grew to
// three call sites doing the identical ~10 lines.
func ensureBackendRunning(port int) (*replcli.Client, error) {
	baseURL := fmt.Sprintf("http://127.0.0.1:%d", port)
	client := replcli.NewClient(baseURL)

	statusCtx, statusCancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	alreadyRunning := client.Status(statusCtx) == nil
	statusCancel()

	if !alreadyRunning {
		if err := spawnDetachedBackend(port); err != nil {
			return nil, err
		}
		if !waitForBackend(client, 10*time.Second) {
			return nil, fmt.Errorf("backend %d portunda ayağa kalkmadı (bkz. %s) / backend did not come up on port %d (see %s)", port, config.DataPath("backend.log"), port, config.DataPath("backend.log"))
		}
	}
	return client, nil
}

// runChatListMode implements `memo -chat <id> -list` — prints an existing
// chat's message history and exits, no new message sent. Deliberately only
// reachable together with -chat (enforced in main(), not here): a bare
// `memo -list` with no chat to list would be meaningless, and requiring
// -chat keeps every one of these scripting flags reading the same way —
// "do X to chat <id>" — rather than some being global and some scoped.
// Thin wrapper around chatListCmd, which does the actual work against a
// *replcli.Client — split apart so tests can exercise chatListCmd directly
// against an httptest server without going through ensureBackendRunning's
// real spawn-a-process path (see cli_remote.go's identical split for the
// same reason).
func runChatListMode(port int, chatID string) int {
	client, err := ensureBackendRunning(port)
	if err != nil {
		fmt.Fprintf(os.Stderr, "FATAL: %v\n", err)
		return 1
	}
	return chatListCmd(context.Background(), client, chatID)
}

func chatListCmd(ctx context.Context, client *replcli.Client, chatID string) int {
	// Read by chat_id: the backend's "active chat" is global, and switching it
	// would move a desktop app on the same backend to this chat.
	msgs, err := client.MessagesFor(ctx, chatID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "FATAL: mesajlar alınamadı / failed to fetch messages: %v\n", err)
		return 1
	}
	if len(msgs) == 0 {
		fmt.Println("Bu sohbette mesaj yok. / No messages in this chat.")
		return 0
	}
	for i, m := range msgs {
		memNote := ""
		if m.MemoryUsed > 0 {
			memNote = fmt.Sprintf(" [memory:%d]", m.MemoryUsed)
		}
		fmt.Printf("%2d. [%s] %s%s\n    %s\n", i+1, m.Timestamp, m.Role, memNote, oneLine(m.Content))
	}
	return 0
}

// runChatMemoryMode implements `memo -chat <id> -memory usage|saved`.
// "usage" is backed by real, already-persisted per-message data
// (ChatMessage.MemoryUsed — see its own doc comment); "saved" is refused
// with an explanation rather than faked, because internal/memory.Store
// entries carry no source-chat-ID field at all (SaveInteraction never took
// one) — there is genuinely no way to answer "which memories did THIS chat
// save" from the current data model, and a command that silently printed
// nothing (or every recently-saved memory regardless of chat) would look
// like it worked while actually lying about what it's showing. Same
// runX/xCmd split as runChatListMode/chatListCmd above, for the same
// testability reason.
func runChatMemoryMode(port int, chatID, query string) int {
	if code, handled := validateMemoryQuery(query); handled {
		return code
	}
	client, err := ensureBackendRunning(port)
	if err != nil {
		fmt.Fprintf(os.Stderr, "FATAL: %v\n", err)
		return 1
	}
	return chatMemoryUsageCmd(context.Background(), client, chatID)
}

// validateMemoryQuery checks query before any network call is made —
// handled=true means the caller should return code immediately (either a
// usage error, or "saved"'s explanatory refusal); handled=false (only for
// "usage") means proceed to the real work.
func validateMemoryQuery(query string) (code int, handled bool) {
	switch query {
	case "usage":
		return 0, false
	case "saved":
		fmt.Fprintln(os.Stderr, "FATAL: -memory saved henüz desteklenmiyor — hafıza kayıtları hangi sohbetten geldiğini tutmuyor (internal/memory.Store'da chat_id alanı yok), bu yüzden \"bu sohbetten kaydedilenler\" sorusu bugünkü veri modeliyle cevaplanamıyor.")
		fmt.Fprintln(os.Stderr, "FATAL: -memory saved is not supported yet — memory entries don't record which chat they came from (no chat_id field in internal/memory.Store), so \"what this chat saved\" can't be answered from today's data model.")
		return 1, true
	default:
		fmt.Fprintln(os.Stderr, "kullanım / usage: memo -chat <id> -memory usage|saved")
		return 1, true
	}
}

func chatMemoryUsageCmd(ctx context.Context, client *replcli.Client, chatID string) int {
	msgs, err := client.MessagesFor(ctx, chatID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "FATAL: mesajlar alınamadı / failed to fetch messages: %v\n", err)
		return 1
	}

	total := 0
	shown := 0
	for i, m := range msgs {
		if m.MemoryUsed <= 0 {
			continue
		}
		total += m.MemoryUsed
		shown++
		fmt.Printf("%2d. [%s] %d hafıza kullanıldı / %d memories used\n", i+1, m.Timestamp, m.MemoryUsed, m.MemoryUsed)
	}
	if shown == 0 {
		fmt.Println("Bu sohbette hiçbir mesajda hafıza kullanılmamış. / No message in this chat used memory.")
		return 0
	}
	fmt.Printf("Toplam / total: %d\n", total)
	return 0
}

// oneLine collapses a message's content to a single, terminal-friendly
// preview line for -list — full multi-line tool output or a long reply
// would otherwise make the listing unreadable.
func oneLine(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	const maxLen = 100
	if len(s) > maxLen {
		return s[:maxLen] + "…"
	}
	return s
}
