# 🏠 Remote Access & Self-Hosting — Step-by-Step Guide

> **Purpose:** Run Memo on a separate machine that stays on 24/7 (a Raspberry Pi, an old mini PC, a rented server) instead of your everyday laptop, and reach it from your phone or desktop. Turn your own computer off, and Memo keeps running.

---

## 🤔 What Does This Do?

Normally Memo runs on whatever computer you opened it on — close that computer, Memo closes with it. But sometimes you want:

- To reach Memo from your phone even when you're not home.
- To be able to turn your laptop off while Memo keeps running.
- To install Memo on a Raspberry Pi, tuck it away somewhere, and forget about it — always on in the background.

That's exactly what this page is about: installing Memo as **just the server** (no desktop window at all) on a separate machine, and connecting to it securely.

**Important:** once connected from your phone or laptop, nothing looks different — same Memo, same interface. The only difference is that your data now lives on that little server machine.

---

## 🚀 Installing — Two Ways

### Option 1: Direct install (Raspberry Pi, Linux, macOS)

On the machine that will act as the server (connect to it via SSH), run this one command:

```bash
curl -fsSL https://download.bugradev.com/get-memo-server.sh | bash
```

It figures out which processor architecture you're on (a Raspberry Pi's ARM chip vs. a regular PC's x86) and grabs the right build automatically. It does **not** install a desktop window — just the background piece.

> 💡 **Want the newest self-hosting features right away?** Everything on this page (the four auth modes, `memo remote`/`memo config`/`memo service`) may not have reached an official release yet — it always lands on the "beta" channel first. To get it immediately, use this instead of the command above: `curl -fsSL https://download.bugradev.com/get-memo-server-beta.sh | bash`. If you're not sure, stick with the regular (stable) command — beta is for testing.

### Option 2: Docker (if you use CasaOS or a similar NAS/home-server dashboard)

If you already know Docker, or use something like CasaOS with an "app store" style interface, Memo also ships a ready-made Docker image — the project's `docker/README.md` walks through it step by step.

---

## 🔒 Security — Choosing Who Can Get In

Once the server is reachable from outside (from outside your home network, or from your phone over the internet), you need to decide **who's allowed in**. Four options:

| Option | What it means | Who it's for |
|---|---|---|
| **Off** | No credential asked at all, anyone can get in | **Never use this on a real network** — testing only |
| **Token** (default) | Each device (phone, laptop) gets its own "access code" | The simplest, safest choice for most people |
| **Password** | Enter a username + password to log in | If you'd rather type a password than copy a code to each device |
| **Token + Password** | Either one works | If you want both options available |

**How a token works, in plain terms:** the first time you connect your phone to the Memo server, it hands you a long, random code (a token). You type that in once, and your phone remembers it from then on. That code is shown to you **exactly once** — you can't see it again later (lose it, and you just generate a fresh one for that device and revoke the old one). Every device has its own code — even if your phone's code got stolen, your laptop's access is unaffected.

There's also automatic protection against guessing: after a handful of wrong attempts in a row, the system blocks further tries for a few seconds, then minutes — trying to guess your password by brute force simply doesn't work in practice.

---

## ⚙️ Keeping It Running in the Background

Right after installing, the setup script asks: "install Memo as a background service?" Say **yes**, and:

- Memo starts on its own every time the computer boots.
- If it ever crashes (rare), it restarts itself automatically.
- A one-time extra command is suggested for it to also start after a reboot without anyone logging in first — run that once and you're done.

---

## 🖥️ Managing It Over SSH

Since there's no desktop app installed on the server, whenever you need to change something, connect over SSH and use these commands:

```bash
memo remote status                    # who can access it right now, which mode is active?
memo remote add-device "My Phone"     # generate a code for a new device
memo service status                   # is the background service running?
```

You'll rarely need these — everything already works once setup finishes. They're only there for later, if you want to add a new device or cut off one that's been lost.

There's also a simple backup web page for emergencies — if your desktop app somehow won't open but you have internet access, typing the server's address into a browser (`http://server-address:8090`) gives you a basic chat interface and status view.

---

## ❓ Frequently Asked Questions

**Q: Is my data still only mine?**
Yes. The server is your own machine (your own Raspberry Pi, your own box) — no data ever goes to Memo's servers or anyone else's.

**Q: When I connect from my phone, does it go over the internet?**
If you're on the same home Wi-Fi, it goes directly over the local network. Away from home, you can use Memo's built-in Tailscale tunnel (turned on once in Settings) for an encrypted connection — no separate service to sign up for.

**Q: What if I accidentally pick "Off" (no credential)?**
Memo shows an impossible-to-miss warning both in desktop Settings and in the terminal output — it never fails silently.

**Q: What if I lose a device?**
You can revoke just that one device's access (`memo remote revoke-device`, or from desktop Settings) — your other devices are unaffected, no need to reset everything.

---

## Linked Notes:
- [[Architecture]]
- [[Developer API Gateway]]
- [[Cloud Sync]]
