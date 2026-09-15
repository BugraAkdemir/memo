# 🐾 Desktop Mascot

> **Frontend files:** `frontend/lib/mascot_main.dart` (separate entry point), `frontend/lib/widgets/mascot_window.dart`, `frontend/lib/widgets/memo_mascot.dart`, `frontend/lib/providers/mascot_provider.dart`
> **Backend:** `internal/app/activity.go`, `internal/models/activity.go`, `internal/webserver/handlers_activity.go`
> **API endpoint:** `GET /api/mascot/activity`
> **Introduced:** v4.5.0

A small animated character that lives on your desktop, separate from the chat window, and shows what Memo is actually doing: thinking, writing, running a tool, or just idling and waiting for you.

---

## A real second window, not a second app

Enable it from Settings → General or the tray, and a small, always-on-top character appears, free to drag anywhere on screen. It's a true second window inside the same running app — same process, same state, same lifecycle (via `desktop_multi_window`) — not a second Memo binary. (The first version of this feature launched as a genuinely separate process; it worked, but doubled memory and felt bolted on — this was reworked before shipping.)

`frontend/lib/mascot_main.dart` is a real, separate Dart entry point (its own `main()`, its own line in the codebase-memory graph's `entry_points`), started as the second window rather than merged into `frontend/lib/main.dart`'s `AppShell`.

---

## It knows what Memo is doing, in real time

Every agent tool call, every plain chat reply, every task-loop turn — whatever channel it comes through (chat, WhatsApp, Telegram, a task list) — feeds one app-wide activity signal (`internal/app/activity.go`, exposed at `GET /api/mascot/activity`) that the mascot polls. Thinking, writing, running a specific tool, generating a reply: its pose changes with it, live.

This same activity signal also powers the terminal REPL's spinner/color output (`internal/replcli/color.go`, `repl.go`) — the mascot and the REPL are two different consumers of the same underlying "what is Memo doing" source of truth.

---

## A plain-language status bubble

Below the character, a small bubble spells out what's happening — "Thinking…", "Writing…", "Running search_web…" — never the model's actual reply text, just a plain status. When a turn finishes, the mascot throws its arms up and the bubble says "Completed!" for a couple of seconds before settling back to idle.

---

## Idle doesn't mean frozen

Left alone, the character blinks, breathes, and throws in an occasional random wave, hop, or little sway at unpredictable intervals, so it reads as alive instead of paused.

---

## Always-on-top, and click-through everywhere else

Getting the character to genuinely stay above every other window required solving a real Wayland limitation — Wayland compositors deliberately refuse to let an app force itself on top (the same security reasoning that blocks focus-stealing) — solved by forcing the whole app onto XWayland. The character's click/drag zone is also shaped to match its real silhouette rather than the square bounding box around it, so the transparent margin around the character doesn't swallow clicks meant for whatever's behind it.

---

## Two looks

Chosen from Settings → General with a live preview of each: the original warm, hand-drawn creature, or a blue-navy pixel-art robot. Both run on the exact same animation system — same moods, same idle flourishes — just a different look.

---

## It talks back in Live Mode too

When Memo is speaking to you in a [[Multimodal Capabilities (Vision and Voice)|Live Mode]] conversation, the mascot's mouth opens and closes in time, with a small sound-wave flourish and a "Speaking…" bubble. Scoped deliberately to speaking only: neither Google Live's nor OpenAI Realtime's client currently surfaces a real "listening" signal from the provider, so none was invented for it.

---

### Linked Notes:
- [[Agent Mode]] — One of the activity sources the mascot reflects
- [[Self-Driving Task Loop]] — Another activity source
- [[Multimodal Capabilities (Vision and Voice)|Live Mode v2]] — The speaking-state animation
- [[Architecture]] — System integration
