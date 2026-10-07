package app

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"

	"memo/internal/api"
	"memo/internal/logx"
)

// SendMessageWithImageStreamTo is SendMessageStreamTo for a message that comes
// with a picture held in memory — a photo sent over Telegram or WhatsApp. Like
// its text-only sibling it targets chatID explicitly (never the chat open in the
// UI) and forceAgent gives the turn tool access regardless of the global toggle.
//
// The picture is stored sealed (persistChatImage) so the chat can show it again;
// it reaches the model as an inline data: URL, the same form a picture attached
// in the app takes, which is also what an image-to-image request is built from
// (image_route.go).
func (a *App) SendMessageWithImageStreamTo(ctx context.Context, chatID, userMsg string, imgData []byte, mime string, forceAgent bool) <-chan api.StreamChunk {
	logx.Printf(">> SendMessageWithImageStreamTo(%s): %q with a %s picture (%d bytes)", chatID, userMsg, mime, len(imgData))

	sm := a.getSessionManager()
	if sm == nil || !sm.SessionExists(chatID) {
		return busyStreamChan(fmt.Sprintf(a.t("sohbet bulunamadı: %s", "chat not found: %s"), chatID))
	}
	if len(imgData) == 0 {
		return busyStreamChan(a.t("⚠️ Görsel boş geldi.", "⚠️ The picture arrived empty."))
	}
	if !strings.HasPrefix(strings.ToLower(mime), "image/") {
		mime = http.DetectContentType(imgData)
	}
	b64 := "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(imgData)

	lockChatID := a.resolveChatID(chatID)
	release, ok := a.lockChatStream(lockChatID)
	if !ok {
		return busyStreamChan(a.busyNotice())
	}

	innerCh, ok := runLockedStreamSetup("SendMessageWithImageStreamTo/setup", release, func() <-chan api.StreamChunk {
		msgs := a.buildMessagesForSession(ctx, chatID, userMsg, []string{b64}, nil)
		stored := persistChatImage("inbound"+imageExtension(mime), imgData)
		sm.AddMessageToSession(chatID, "user", userMsg, stored, "")
		return a.routeStream(ctx, msgs, userMsg, stored, "", chatID, forceAgent)
	})
	if !ok {
		return busyStreamChan(a.busyNotice())
	}

	out := make(chan api.StreamChunk, 128)
	go func() {
		defer close(out)
		defer release()
		defer recoverPanic("forwardStream")
		forwardStreamReleasing(ctx, innerCh, out, release)
	}()
	return out
}
