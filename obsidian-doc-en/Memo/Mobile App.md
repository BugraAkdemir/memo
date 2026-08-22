# 📱 Memo Mobile App — Step-by-Step Guide

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

### 2. Build & Run Mobile App

```bash
cd mobile
flutter run
```

Your phone must be connected via USB with developer mode enabled.

### 3. Connection Screen

When the app opens, you'll see a connection screen:

| Field | What to Enter |
|-------|--------------|
| **Server Address** | Your computer's IP + port: `http://192.168.1.42:8090` |
| **Token (optional)** | The access token if you set one in Settings |

**LAN Auto-Discovery:** If you don't know the IP, tap the **Scan** button. The phone scans all addresses on your network, finds Memo, and fills in the address automatically.

4. Tap **Connect**.

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

The mobile app has a calendar tab. It shows:

- **Monthly grid view** — days with events have dots
- Tap a day to see its events
- Add events manually
- Long-press an event to delete it
- Change the reminder lead time

## ⏰ Routines on Mobile (v3.3.3) — REMOVED in v3.9.0

> **⚠️ Out of date as of v3.9.0:** routine delivery to the phone was removed entirely. The mobile app no longer polls `/api/routines/mobile-ready` — the endpoint was deleted from the backend because there is no actively used mobile app to receive it, and it now returns 404. Routines are now created, managed, and delivered through **WhatsApp or Telegram self-chat** instead (see [[WhatsApp Entegrasyonu]] / the Telegram page). Calendar reminders are unaffected — they come from the calendar system and still arrive as locally pre-scheduled notifications.

Routines work on mobile too, not just desktop. Since the mobile app has no push channel, it polls `/api/routines/mobile-ready` to pre-schedule a **real, pre-scheduled local notification** ahead of a routine's fire time — it still arrives even if the app isn't open. Notification text follows the language the routine was created in (fixed in v3.3.3 — it previously always came out in Turkish regardless of the app's own language setting). See [[Proactive Learning and Calendar]].

## 🌍 Full Localization (v3.3.3)

The mobile app is now fully localized (Turkish/English), matching the desktop app, with a language toggle available both in Settings and on the pre-pairing connect screen.

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
