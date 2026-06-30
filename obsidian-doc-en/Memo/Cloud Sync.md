# ☁️ Cloud Sync — Step-by-Step Guide

> **Purpose:** Automatically back up all your Memo data (memory, chats, settings) to your Google Drive. If your computer crashes or you move to a new machine, nothing is lost. And Google can't read your data — everything is locked with a passphrase only you know.

---

## 🤔 What Does This Do?

Think about it: you've used Memo for 3 months. It knows you, knows when you work, remembers conversations from weeks ago. Then your computer dies. All that memory — gone.

Cloud sync prevents this.

Every 50 messages (configurable), Memo quietly does this in the background:
1. Packs your data into an archive
2. Locks it with your passphrase
3. Uploads it to your Google Drive

Result: an encrypted backup sits in your Google Drive. Nobody can open it — only you, with your passphrase.

---

## 🔐 Encryption — The Simplest Explanation

```
Your data → [LOCKED WITH PASSPHRASE] → Google Drive
                                          ↓
                                     Google can't read this.
                                     Only the passphrase holder can unlock it.
```

- **Without passphrase:** The file in Google Drive is meaningless, scrambled data. Even Google can't make sense of it.
- **With passphrase:** The file decrypts, Memo reads your data, everything is restored.

> ⚠️ **If you forget your passphrase, there is NO recovery.** Write it down somewhere safe. Without it, your cloud backups are locked forever.

---

## 📦 What Gets Backed Up?

| Data | Contents |
|------|----------|
| **Memory (memory.db)** | All RAG memories, everything it knows about you |
| **Chats (sessions/)** | All conversation history |
| **Provider config** | Your API keys (already encrypted on disk) |
| **Orchestra config** | Which model is assigned to which role |
| **Agent permissions** | Which tools you've allowed the agent to use |
| **Learning patterns** | When you work, your habits |
| **Mood state** | The emotion engine's current score |

---

## 🚀 Setup — Step by Step

### Step 1: Create a Google Cloud Project

Memo needs permission to write to your Drive. You create a small project to grant this. **Free, takes 5 minutes.**

1. Go to [Google Cloud Console](https://console.cloud.google.com)
2. Create a new project (name doesn't matter, "Memo Sync" works)
3. Left menu → **APIs & Services → Library**
4. Search for **Google Drive API**, enable it
5. Go to **Credentials** page
6. Click **Create Credentials → OAuth Client ID**
7. Application Type: **Desktop App**
8. Give it a name (e.g. "Memo Desktop"), click **Create**
9. You'll see a **Client ID** and **Client Secret** — copy both

> These two codes let Memo say "I'd like permission to write to your Drive." Don't share them, but they're not super secret — they just identify the app.

### Step 2: Enter Credentials in Memo

1. Open Memo
2. Go to **Settings → Cloud Sync**
3. Fill in:
   - **Client ID:** The long code from Step 1
   - **Client Secret:** The other long code from Step 1
   - **Passphrase:** A strong password you choose
     - At least 8 characters
     - Mix of uppercase, lowercase, numbers
     - **Don't forget this!** Write it down somewhere.
4. Click **Save**

### Step 3: Log In to Google

1. Click **Login with Google**
2. Your browser opens — choose your Google account
3. Google asks "Memo wants to access these files" — click **Allow**
4. Close the browser, return to Memo
5. You'll see "Connected"

### Step 4: Take Your First Backup

1. Click **Sync Now**
2. Wait — your data is being encrypted and uploaded to Google Drive
3. When you see "Sync complete", you're done
4. **From now on it's automatic** — backups happen every 50 messages without you doing anything

---

## 🔄 Restore on Another Computer

You got a new computer. You installed Memo. To bring back your old memory:

1. **Enter the same Client ID and Client Secret** (like Step 2)
2. **Enter the same Passphrase** — it MUST match, or data won't decrypt
3. **Login with Google**
4. Click **Pull from Cloud**
5. Memo downloads the latest backup, decrypts it with your passphrase, restores everything
6. Everything is back — your memory, chats, settings, just like before

---

## ⚙️ Settings

| Setting | What It Does | Default |
|---------|-------------|---------|
| **Interval (messages)** | How many messages trigger a backup | 50 |
| **Passphrase** | Your encryption password — lose it, lose the data | (empty = machine key) |

> If you leave the passphrase empty, Memo uses an automatic machine-specific key. This works on the same computer but **can't be moved to another machine.** Setting a passphrase is strongly recommended.

---

## ❓ FAQ

**Q: Can Google read my data?**
No. Data is encrypted before it leaves your computer. Google only sees an encrypted file — it can never read the contents.

**Q: What happens when I'm offline?**
Nothing. Sync only runs when you have internet. It skips when offline and resumes when connected.

**Q: I want to change my passphrase.**
Enter a new passphrase in Settings, save. Future backups use the new passphrase. Old backups stay encrypted with the old passphrase — you'll need the old passphrase to restore them.

**Q: How many backups are kept?**
The last 3. Older ones are automatically deleted to save Drive space.

---

## Related Notes:
- [[Data Layer and Persistence]]
- [[Architecture]]
