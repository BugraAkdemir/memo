# 📱 Memo on Your Phone — Step-by-Step Guide

> **Changed in 2026-09:** there is no separate mobile app any more. The
> standalone `mobile/` project was retired and the **same** Flutter client as
> the desktop build (`frontend/`) now has Android and iOS targets, so the phone
> gets the real screens — chat, agent mode, calendar, routines, the model
> store, settings — laid out for a narrow viewport. Everything below describes
> that client.

> **What is this?** Connect to Memo from your phone. Chat, check your calendar, get reminders. All AI processing happens on your desktop — the phone is just a "remote control." Your phone won't heat up, drain battery, or eat mobile data.

---

## 🤔 What Does This Do?

Imagine: Your computer is in your room, Memo is running on the desktop. You're in the kitchen and want to ask something. Without walking to the computer, you type on your phone. Memo processes everything on the desktop and sends the answer to your phone.

Or you're out. You connect to Memo remotely via ngrok/Tailscale. You check your calendar, add an event, get a reminder.

The phone only displays what you type and the answers you receive. All heavy lifting — LLM inference, RAG search, embedding — happens on the desktop.

---

## 📱 Setup — Step by Step

### Prerequisites

- Memo desktop app **must be running**
- Phone and computer **on the same Wi-Fi** (for LAN connection)
- Or ngrok/Tailscale **active** on the desktop (for remote connection)

### 1. Desktop Preparation

Nothing special needed in Memo settings. Just make sure it's running. To find your computer's IP address:

```bash
# Linux / macOS
ip addr show | grep "inet " | grep -v 127
# or: ifconfig | grep "inet "

# Windows
ipconfig
```

Example output: `192.168.1.42` — this is your computer's IP address.

If connecting remotely, go to Settings → Remote Access and enable ngrok or Tailscale.

### 2. Build & Run on the Phone

```bash
cd frontend
flutter run          # phone connected via USB, developer mode on
# or, to build an installable debug APK:
flutter build apk --debug
```

CI also builds both targets on every push (`build-android.yml` produces a debug
APK artifact; `build-ios.yml` builds iOS unsigned, since this project has no
signing certificates).

### 3. First Run — Server Address

On a phone the setup wizard opens with a **Connect to your server** step first,
because a phone never runs Memo's backend itself and so has no address to
default to:

| Field | What to Enter |
|-------|--------------|
| **Server address** | Your computer's IP + port: `192.168.1.42:8090` (the scheme is filled in for you) |
| **Access token** | The token if you set one in Settings → Remote Access |

Tap **Test connection** — a green line means Memo answered, and the rest of the
wizard (persona, model, preferences) then works normally.

A Tailscale address can be entered directly: a `*.ts.net` host is recognised and
gets `https://` with no port appended, since Funnel serves over standard 443.

**No LAN auto-discovery.** The retired client had a "Scan" button that probed
every address on the subnet; it was not carried over. Type the address, or use a
Tailscale name that doesn't change.

---

## 🏠 Same Wi-Fi Connection (LAN)

The simplest method. Phone and computer just need to be on the same network.

```
Phone ←──── Wi-Fi ────→ Computer (Memo running)
   ↓                        ↓
 192.168.1.100          192.168.1.42:8090
```

1. Find your computer's IP (using the command above)
2. Enter this IP plus `:8090` in the mobile app
3. Connect

> This method **only works on the same home/office network.** You can't connect from outside.

---

## 🌍 Remote Connection (ngrok / Tailscale)

You're not home, want to reach Memo.

### Option 1: ngrok (easiest)

1. Go to [ngrok.com](https://ngrok.com), create a free account
2. Copy your auth token
3. In Memo: **Settings → Remote Access → Ngrok**
4. Paste the token, toggle **Ngrok Active** on
5. Memo will give you a URL: `https://abc123.ngrok.io`
6. Enter this URL in the mobile app

```
Phone ←──── Internet ────→ ngrok server ────→ Your computer
```

> Free ngrok generates a new URL each time you start it. You'll need to update the mobile app each time.

### Option 2: Tailscale (stable URL)

Tailscale-based remote access **graduated out of Beta in v3.3.4** — no longer needs the Beta Features switch on either platform.

1. Sign up at [Tailscale](https://tailscale.com)
2. In Memo: **Settings → Remote Access → Tailscale**
3. **One-click login (v3.3.4):** an interactive login flow — no auth key to paste anymore
4. Set a hostname (e.g. `memo-home`)
5. Install the Tailscale app on your phone too
6. In the mobile app, connect to `http://memo-home:8090`

```
Phone ←── Tailscale network ──→ Computer
(Tailscale app)                 (Memo + embedded Tailscale)
```

> With Tailscale, the URL **stays the same forever.** Set it once, never change it. As of v3.3.4, the mobile app also auto-reconnects with the saved URL on a cold start, and shows a real error message instead of a raw exception dump when a connection drops.

---

## 🎯 Calendar Tab

The calendar tab shows:

- **Monthly grid view** — days with events have dots
- Tap a day to see its events
- Add events manually
- Long-press an event to delete it
- Change the reminder lead time

## 🔔 Notifications

**Calendar reminders arrive as real notifications**, scheduled with the OS
itself rather than pushed over a connection — so they fire even when Memo is
closed and the phone hasn't heard from the backend since. They are re-armed
whenever the calendar loads and when the app is brought back to the foreground.
Permission is asked for from the setup wizard's preferences step, not silently
at first launch.

One known gap: an event the assistant adds while you never opened the calendar
tab is not armed until you do (see KNOWN_ISSUES M32).

**Routine notifications do not exist** — that path was removed in v3.9.0 along
with the `/api/routines/mobile-ready` endpoint it polled. Routines are created,
managed and delivered through **WhatsApp or Telegram self-chat** instead (see
[[WhatsApp Integration]]).

## 🌍 Localization

Fully localized (Turkish/English) — it is the same `l10n.dart` the desktop
build uses, so the two cannot drift apart.

## 🚫 What Isn't on a Phone

- **Live Mode's native realtime engines** (Google Live, OpenAI Realtime) — their
  streaming audio playback is Linux-only. The voice button falls back to the
  discrete record → transcribe → reply → speak loop, which does work.
- **The desktop mascot** and the **system tray**, neither of which a phone has.
- **CLI installation** — no shell, no home directory to install into.

Things that *look* desktop-only but work fine, because they are server-side
REST calls: the model store (the server downloads the GGUF onto its own disk),
GPU info (it reports the server's GPU), and the llama.cpp installer.

---

## 🔐 Token Protection

You can password-protect the connection:

1. Desktop Memo → **Settings → Remote Access**
2. Enter a password in the **Access Token** field
3. Enter the same token when connecting from the mobile app

Without the token, connections are rejected. **Highly recommended** if you've opened Memo to the internet via ngrok.

---

## ❓ FAQ

**Q: Does it work without internet?**
Yes. LAN connection doesn't need internet — same Wi-Fi is enough. Internet is only needed for ngrok/Tailscale remote connections.

**Q: Do I need to run models on my phone?**
No! All AI processing happens on the desktop. The phone just sends text and displays the response. Your phone can be old, slow, low battery — doesn't matter.

**Q: Can I chat from both desktop and phone at the same time?**
Yes, you can connect to the same session from both. But if you send messages from both at the same time, things might get mixed up — use one at a time.

---

## Related Notes:
- [[Remote Access]]
- [[Proactive Learning and Calendar]]
- [[Architecture]]
