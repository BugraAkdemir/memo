package app

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"time"
)

// outboxTTL is how long a share_file download link (the frontend/normal-
// chat delivery path — see DeliverFile) stays valid. Generous enough to
// cover "I'll click it in a bit", short enough that a backend left running
// for weeks doesn't accumulate an unbounded number of stale entries/files.
const outboxTTL = 24 * time.Hour

// outboxEntry is one file staged for download via handleOutboxDownload.
// isTempFile mirrors FileSender's own isTempFile parameter — only ever
// true for a zip share_file created itself, never for the user's real
// file — and gates whether expiry cleanup is allowed to delete the
// underlying file: deleting a temp zip nobody downloaded is a cleanup,
// deleting the user's own real file because a download link expired would
// be a correctness bug (see registerOutboxFile/GetOutboxFile's doc
// comments).
type outboxEntry struct {
	path       string
	filename   string
	expiresAt  time.Time
	isTempFile bool
}

// DeliverFile is the share_file agent tool's backing implementation
// (internal/agent/tools/sendfile.go's FileSender) — resolves where fullPath
// actually goes purely from ctx, exactly like CreateRoutineFromChat
// resolves its own delivery target (see selfChatSourceFromContext's doc
// comment for the shared reasoning): the model never supplies a
// destination.
//
//   - Called from WhatsApp/Telegram self-chat: sent (or attempted) directly
//     as a document to that exact surface/chat. consumed=true whether the
//     send succeeded or failed — there is no retry path either way, so a
//     temp zip the caller created has nothing left to be used for and is
//     safe to delete immediately. isTempFile isn't needed on this branch:
//     the caller already deletes based on consumed alone.
//   - Called from normal chat (no self-chat source on ctx, which also
//     covers the desktop/web frontend): staged in the in-memory outbox and
//     a relative download link is returned as the tool's reply text, which
//     the chat UI renders as a clickable link. consumed=false — the file
//     must keep existing on disk until either it's downloaded or outboxTTL
//     expires it, at which point isTempFile decides whether expiry is
//     allowed to actually delete it from disk.
func (a *App) DeliverFile(ctx context.Context, fullPath, displayName string, isTempFile bool) (string, bool, error) {
	src, hasSource := selfChatSourceFromContext(ctx)

	if hasSource && src.WhatsApp {
		if a.waClient == nil {
			return "", true, errors.New(a.t("WhatsApp bağlı değil", "WhatsApp is not connected"))
		}
		if _, err := a.waClient.SendDocument(ctx, src.WhatsAppJID, fullPath, displayName); err != nil {
			return "", true, err
		}
		return fmt.Sprintf(a.t("Dosya WhatsApp'tan gönderildi: %s", "File sent via WhatsApp: %s"), displayName), true, nil
	}

	if hasSource && src.Telegram {
		a.tgMu.Lock()
		client := a.tgClient
		a.tgMu.Unlock()
		if client == nil {
			return "", true, errors.New(a.t("Telegram bağlı değil", "Telegram is not connected"))
		}
		if err := client.SendDocument(ctx, src.TelegramChatID, fullPath, displayName); err != nil {
			return "", true, err
		}
		return fmt.Sprintf(a.t("Dosya Telegram'dan gönderildi: %s", "File sent via Telegram: %s"), displayName), true, nil
	}

	url := a.registerOutboxFile(fullPath, displayName, isTempFile)
	return fmt.Sprintf(a.t("[%s indir](%s)", "[Download %s](%s)"), displayName, url), false, nil
}

// registerOutboxFile stages path for download under a fresh random token
// and returns the relative URL to fetch it — relative, not absolute,
// because App has no reliable way to know which host/port a remote client
// (LAN, ngrok, Tailscale) actually used to reach this backend; the frontend
// already knows its own api base URL and resolves the relative link against
// it (see chat_message_list.dart's onTapLink).
//
// Pruning expired entries here (in addition to GetOutboxFile's own lookup-
// time check) is what actually reclaims a temp zip nobody ever downloaded —
// without this, only entries someone happens to look up again would ever
// get cleaned, and a zip that's simply never clicked would sit in the
// outbox map (and on disk, since deleteIfTemp below only runs for entries
// this loop or GetOutboxFile actually visits) until the process restarts.
func (a *App) registerOutboxFile(path, filename string, isTempFile bool) string {
	a.outboxMu.Lock()
	defer a.outboxMu.Unlock()
	if a.outbox == nil {
		a.outbox = make(map[string]outboxEntry)
	}
	now := time.Now()
	for tok, entry := range a.outbox {
		if now.After(entry.expiresAt) {
			deleteIfTemp(entry)
			delete(a.outbox, tok)
		}
	}
	token := randomOutboxToken()
	a.outbox[token] = outboxEntry{path: path, filename: filename, expiresAt: now.Add(outboxTTL), isTempFile: isTempFile}
	return "/api/files/outbox/" + token
}

// GetOutboxFile resolves a download token to a path/filename — the
// handleLlamaInstall-webserver counterpart of registerOutboxFile above.
// Expired tokens are treated as not found; deleteIfTemp reclaims the
// underlying file at that point too, same as registerOutboxFile's own
// pruning loop, but ONLY when the entry is a temp file share_file itself
// created — never the user's real file, which must survive a link
// expiring exactly as it would have if share_file had never been called.
func (a *App) GetOutboxFile(token string) (path, filename string, ok bool) {
	a.outboxMu.Lock()
	defer a.outboxMu.Unlock()
	entry, found := a.outbox[token]
	if !found {
		return "", "", false
	}
	if time.Now().After(entry.expiresAt) {
		deleteIfTemp(entry)
		delete(a.outbox, token)
		return "", "", false
	}
	return entry.path, entry.filename, true
}

// deleteIfTemp removes entry's underlying file from disk, but only when it
// was share_file's own temp zip — see outboxEntry's doc comment for why
// this distinction is load-bearing, not cosmetic.
func deleteIfTemp(entry outboxEntry) {
	if entry.isTempFile {
		os.Remove(entry.path)
	}
}

// fileToolAdapter wraps *App to satisfy tools.FileSender (name mismatch
// only: DeliverFile already matches, kept as a thin type for symmetry with
// routineToolAdapter/waToolAdapter).
type fileToolAdapter struct{ a *App }

func (f fileToolAdapter) DeliverFile(ctx context.Context, fullPath, displayName string, isTempFile bool) (string, bool, error) {
	return f.a.DeliverFile(ctx, fullPath, displayName, isTempFile)
}

func randomOutboxToken() string {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return hex.EncodeToString([]byte(fmt.Sprintf("%d", time.Now().UnixNano())))
	}
	return hex.EncodeToString(buf)
}
