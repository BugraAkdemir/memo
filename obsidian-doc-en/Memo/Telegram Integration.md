# Telegram Integration

> **Package:** `internal/telegram/` (`client.go`, `store.go`)
> **API endpoints:** `/api/telegram/status`, `/api/telegram/connect`, `/api/telegram/stop`, `/api/telegram/disconnect`
> **Added:** v3.9.0

Memo can connect to a Telegram bot as a second chat surface, alongside WhatsApp. Unlike WhatsApp's whatsmeow integration (which emulates a full WhatsApp Web client with visibility into your entire existing account), a Telegram bot can only ever see messages sent directly to it — there's no equivalent of "read my other chats" without the much heavier MTProto user API (phone-number login, not a bot token). That scope difference is deliberate: this package exists to give a bot token from @BotFather a way to talk to Memo, not to mirror WhatsApp's contact/group/history breadth.

## Setup

1. Talk to [@BotFather](https://t.me/BotFather) on Telegram, create a bot, get its token.
2. Paste the token into Settings → Telegram (or `POST /api/telegram/connect`).
3. Memo starts long-polling the Bot API (`internal/telegram/client.go`) for new messages.

## Owner Lock

Since anyone who finds a bot's username can message it, Memo needs its own access-control boundary — there's no QR-pairing step like WhatsApp has. `shouldReplyToTelegram` (`internal/app/telegram.go`) locks in whoever sends the **first** message as the bot's permanent owner (`tgStore.SetOwner`), then silently ignores every other sender from then on. This is the entire authorization model for the integration — there is no shared/multi-user Telegram bot mode.

## Self-Chat Assistant

Once the owner is linked, messaging the bot gets you the same assistant capability as WhatsApp's self-chat: chat, memory, and agent tools, without opening Memo itself. `handleTelegramMessage`/`handleTelegramCommand` (`internal/app/telegram.go`) mirror `handleWhatsAppSelfChatMessage`/`handleWhatsAppSelfChatCommand` — a leading-slash command is handled directly, anything else goes through the normal background chat session (`sm.NewBackgroundChat`, cached per-run as `a.tgSelfChatSessionID`).

- **Routines via chat**: ask in plain language and Memo creates/lists/cancels a routine with the same `create_routine`/`list_routines`/`cancel_routine` agent tools available in-app.
- **Permission answers**: `routeTelegramPermissionAnswer` intercepts replies to a pending agent-tool permission prompt sent as a Telegram message, same idea as WhatsApp's flow.

## Technical

- **Storage**: `internal/telegram/store.go` — `OwnerChatID` (0 = not yet linked) plus message history, isolated from WhatsApp's own SQLite database.
- **Long-polling, not webhooks**: simpler to self-host (no public HTTPS endpoint required), at the cost of slightly higher latency than a webhook push.
- **Status**: functionality is covered by unit tests (`client_test.go`, `store_test.go`); not yet confirmed against a live bot by the user clicking through the real Telegram app end-to-end.

## Linked Notes:
- [[WhatsApp Integration]] — the other self-chat surface, same assistant pattern
- [[Agent Mode]] — the tool-calling loop both self-chat surfaces route into
- [[Backend (Go) Architecture]] — package structure
