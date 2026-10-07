# Memo UI — Feature Inventory

---

## 1. NAVIGATION (app_shell.dart)

- AppShell → NavRail (64px) + IndexedStack
- **NavRail**: Logo, Chat, Agent, Models, WhatsApp, Calendar, Routines, Developer, Swarm (beta-gated, hidden unless Settings → Beta Features is on and the platform supports it — not macOS), Settings (dialog)
- Active tab: filled icon, accent color
- SetupWizardOverlay (first launch)
- LlamaInstallerOverlay (download progress)
- VersionBanner (update notification)
- Theme: light/dark toggle (System, Light, Dark)

---

## 2. CHAT SCREEN (chat_screen.dart)

### 2.1 Chat Sidebar (chat_sidebar.dart)
- Width: 260px
- Search bar (filter by title)
- "New Chat" button
- Chat list: title, preview, timestamp, delete on hover
- Active chat highlight
- A small spinner on every chat that is generating a reply right now (and a "just finished" mark right after) — background tasks, WhatsApp/Telegram replies, other browser tabs and Claude Code/Codex sessions included (v4.6.0, `GET /api/chats/streaming`)
- Empty state: "No chats yet"

### 2.2 Top Bar (_ChatTopBar)
- Title + badges (incognito, agent, agent project)
- Badge styles: icon + label, colored bg 12% alpha
- Action buttons: Undo (agent), WhatsApp toggle, Export (markdown)
- **Model picker** (`widgets/model_picker.dart`, v4.6.0): a panel, not a popup menu. Height capped to the room below the button, scrolls inside, search box once there are more than seven rows, foldable vendor sections, model-family logos, readable names from `prettyModelName` ("Gemini 3.7 Flash" + a "High" tag; the raw id is what is stored and sent). Subscriptions models carry a remaining-allowance badge (`quota_badge.dart`) and time to refill; the picker re-reads quota 2.5 s after opening and every 50 s. `chat_input.dart`'s own switcher still lists raw ids

### 2.3 Messages Area
- WelcomeView (empty state): Logo, tagline, feature cards, quick actions
- ChatMessageList: scrollable, auto-scroll, RepaintBoundary per bubble
- **Live progress line** (`stream_progress.dart`, v4.6.0): while a turn runs, the current phase and a one-second elapsed timer ("The model is thinking · 0:45"); orange "no word from the server" after 30 s of silence. The backend's 10-second `heartbeat` chunks keep the idle timeout from firing during a quiet thinking phase
- **Usage-limit card** (`quota_notice_card.dart`, v4.6.0): shown when a turn failed because the allowance ran out — countdown to the refill, an "automatically continue when it resets" checkbox (default on, device preference `memo_quota_auto_continue`) and "Continue now"; at the refill time `continue` is typed into the chat. A `quota_low` marker shows a one-off low-allowance warning. In memory only, one card per chat

### 2.4 Message Bubbles (chat_message_list.dart)

**User bubble:**
- Right-aligned, accentMuted bg (6% gold)
- Image preview (if hasImage): Image.file, 480px max, rounded
- Markdown content (MarkdownBody)
- Timestamp (always visible)

**Assistant bubble:**
- Left-aligned, avatar (M logo)
- bgPanel bg, borderSoft border
- Thinking toggle (collapsible section)
- Agent event cards (streaming tool calls)
- Markdown content
- Timestamp + copy button (on hover)

**Streaming bubble:**
- Real-time content from streamingContentProvider
- Thinking from streamingThinkingProvider

**Typing indicator:**
- 3 animated pulsing dots

### 2.5 Chat Input (chat_input.dart)

**Image picker:**
- Button → FilePicker (image type) → sets _pickedImagePath
- Preview row: 60x60 thumbnail, filename, close button
- Does NOT send immediately — user types prompt + sends manually

**File picker:**
- Button → FilePicker (any type) → sets _pickedFilePath
- Same preview + send pattern

**Text field:**
- Multiline, max 160px height
- "/" triggers prompt template popup
- Filtered list, arrow navigation, Enter selects

**Send/Stop button:**
- Gold accent (idle) → red (sending) via AnimatedContainer
- Stop icon when sending, Send icon when idle

**Send logic:**
- If image/file picked → sendFile(text, path) (streaming or non-streaming)
- If text only → sendMessage(text)
- WhatsApp mode → _sendWhatsApp(text)

---

## 3. MODEL STORE SCREEN (model_store_screen.dart)

### Current tabs:
- Discover, Local Models, Downloading (horizontal tab bar)

### 3.1 Discover Tab
- Search bar (HuggingFace search)
- Filter chips: All, Vision, Text, Embedding, Tool
- Model cards grid (2 columns)
- Card: name, author, tags, downloads, likes, download button
- Loading: skeleton cards
- Empty: "No models found"
- Error: retry button

### 3.2 Local Models Tab
- Search + sort bar
- Local model cards:
  - Filename + repo ID
  - Tags: Vision (backend mmproj_path), Embedding, Think, Tool, Text
  - File size (GB/MB format)
  - Delete button
  - Actions: Start (play icon), Config (gear), Show in Dir (folder)
  - Status: running/stopped/loading/error
- Models filtered: mmproj/multimodal files hidden
- Model config dialog: ctx_size, port, gpu_layers, start/stop

### 3.3 Downloading Tab
- Active download: progress bar, speed, ETA, cancel
- Empty: "No active downloads"

---

## 4. AGENT SCREEN (agent_screen.dart)

- Agent mode toggle
- New Agent Chat: picks project folder
- Agent chat list
- Agent messages: tool execution cards with status
- Permission requests: dialog (allow/deny/always)
- Undo button for edits
- **Code Mode sub-mode chip** (bottom engine strip, `engine_strip.dart`): shows the active sub-mode (Plan/Auto/Build); cycle with **Ctrl+Tab** anywhere in the app, or tap the chip
- **Plan approval**: when a Plan sub-mode turn finishes, Memo asks in plain chat whether to proceed to Build or Auto (skipped — chains straight to Build — if the global auto-permission toggle is already on)

## 4.1 TASK DETAIL SCREEN (task_detail_screen.dart)

- Self-Driving task list detail view: live phase/progress, plan-approval card (`_PlanApprovalSection`) for planner-mode lists, escalation banner
- Live activity: tool calls, sub-agent turns (`[coder]`/`[analyzer]`/`[reviewer]`/`[test-runner]`), "model is generating" indicator during long silent LLM calls
- Pause/resume/cancel/skip controls for a running task list

---

## 5. WHATSAPP SCREEN (whatsapp_screen.dart)

- Connection status: connected/disconnected/connecting
- QR code display for pairing
- Contact list with avatars
- Message history + search
- WhatsApp-style input

---

## 5.1 CALENDAR SCREEN (calendar_screen.dart)

- Month view + event list, add/edit events

## 5.2 ROUTINES SCREEN (routines_screen.dart)

- List of scheduled Routines (schedule + prompt or agent config)
- Plain-language description → parsed into a routine config
- Desktop; mobile shows real pre-scheduled local notifications

## 5.3 DEVELOPER SCREEN (developer_screen.dart)

- Developer API Gateway, both wire formats: Anthropic-compatible (for Claude Code) and OpenAI-compatible (`GET /v1/models`, `POST /v1/chat/completions`) — Base URL, model list (`type/model-id`), token, live request log
- API key requirement now **enforced** for any non-loopback caller on both endpoints (v4.5.0); optional memory integration
- **Settings › Subscriptions**: sign in with an Antigravity, Claude or Codex account; every model the account offers appears in the chat's top-right model selector, `/model` and the local `/v1` gateway (`subs/<model>`), no separate API key

## 5.3.1 BROWSER PANE (widgets/agent/browser_pane.dart) — new in v4.6.0

- Opens next to the chat; shows the live screenshot of the isolated Chromium session Memo drives (`browser_*` tools), repainted after every page-changing step
- Works both ways, with Agent Mode off too: an address bar (no `https://` needed), a click on the screenshot clicks the same spot on the page (mapped through the PNG's real pixel size), a keyboard row to type into the focused field, scroll
- Resizable by dragging; full width on narrow (phone-width) screens
- Frames arrive as `browser_frame` SSE chunks and are never part of the chat history; the session endpoints are `/api/browser/session/*`

## 5.4 SWARM SCREEN (swarm_screen.dart) — BETA

- Host: create room, room code, add/reorder/remove joined workers, set each worker's compute share
- Join: enter a room code to lend compute without downloading the model
- Gated behind Settings → Beta Features; not shown on macOS

## 5.5 LIVE MODE V2 — VOICE ICON (chat_input.dart)

- A small icon next to the chat input box (not a separate screen/tab)
- Native audio-to-audio conversation via Google Live or OpenAI Realtime, delegate (into an existing chat) or standalone modes; falls back to local whisper.cpp transcription + Piper TTS when no native engine is configured
- One-directional barge-in; mid-session memory refresh (not just once at session start)
- Clear failure messages when a real voice engine can't start (no engine selected / incomplete config / background chat session failing to open) instead of a silent self-echo
- No echo cancellation yet (known limitation)

## 5.6 MASCOT WINDOW (mascot_window.dart / memo_mascot.dart) — new in v4.5.0

- A separate, always-on-top, frameless window (same process, `desktop_multi_window`) showing an animated character reflecting Memo's live activity — not a screen inside `AppShell`
- Enabled/disabled and skin-selected from Settings → General, with a live preview of each skin
- Poses driven by `/api/mascot/activity`: thinking, writing, running a specific tool, a distinct "Completed!" moment, idle flourishes (blink/breathe/occasional wave)
- Speaks in time during Live Mode (mouth animation + "Speaking…" bubble) — no "listening" pose, since neither voice engine currently reports one
- Click/drag zone shaped to the character's silhouette, not its bounding box; forced onto XWayland on Linux so always-on-top actually works under Wayland

---

## 6. SETTINGS DIALOG (settings_dialog.dart)

- Reorganized (v3.3.4) into a **searchable, grouped rail** with a search box up top, replacing the old flat row of ~20 tabs
- Left panel: grouped tab rail
- Right panel: content area
- Includes (non-exhaustive): General (incl. Minimal Mode, per-sub-feature overrides, mascot toggle + skin picker), Providers, CLI Connections (Claude Code/Codex install check), Llama, Memory, Cloud Sync, Identity, Orchestra, Agent Permissions, **Code Sub-Mode Prompts** (edit the Plan/Auto/Build system prompts), Skills, Learning (Proactive), Live Mode (engine + voice config), Stats (Usage), Beta Features, Remote Access, Accounts (self-hosted multi-user), Backup/Restore, Report Bug, About
- Tabs (original set, still present in some form):

### General
- Theme selector (System/Light/Dark)
- Language selector (Türkçe/English)
- Streaming toggle
- Beta features toggle
- Incognito mode toggle
- Desktop mascot toggle + skin picker (live preview), v4.5.0

### Providers
- Provider cards: name, active status
- Configure, Test, Delete buttons
- Add Provider button — providers with a genuine free tier carry a green badge and sit in their own group ("Have a free model"); OpenRouter, Kilo Code, OpenCode Zen and Cline offer a **Pick a free model** button that selects a working $0 model in one click (v4.6.0). Cline is new in v4.6.0
- Active provider indicator

### Llama
- Binary path + Browse
- Models directory + Browse
- GPU Layers (slider)
- Engine mode (CPU/NVIDIA/AMD dropdown)
- Context size (text field)
- Temperature (slider)

### Memory
Five sections switched with a pill bar, each pill carrying a live count (v4.6.0):
- **Settings** — memory enabled toggle, Top K results, min similarity, memory files list + delete
- **Known Facts** — every pinned fact, editable in place; checkboxes plus a select-all / deselect-all bar for bulk delete
- **Conversation History** — the ordinary (non-pinned) memories, paginated, same selection tools
- **Analytics** — usage analytics
- **Debug** — debug search
- Every delete asks for confirmation and reports how many records it removed

### Subscriptions (`subscriptions_tab.dart`, v4.6.0)
- One sign-in per vendor — Antigravity, Claude, Codex; the browser opens (Memo opens it itself), the account's email is shown, sign out any time
- The account's live model list grouped by vendor, with a remaining-allowance badge and time to refill; re-read every 50 s while the tab is open. The list stays empty for ~30 s after a sign-in, so the tab keeps re-reading until models arrive
- A note about third-party-client risk is shown on the page

### Skills (`skills_tab.dart`)
- Lists, installs and removes skills. Turning a skill on or off happens inside the chat — activation is per chat (v4.6.0)

### Usage Stats (`stats_tab.dart`)
- KPI cards, 30-day chart, per-model breakdown, and the **Prompt Cache** panel (v4.6.0): read from cache / written to cache / full-price input, hit ratio, "N cached" badges per model and category; "not reported" when the provider sends no cache figure

### Cloud Sync
- Google Drive auth status
- Sync enabled toggle
- Auto-sync interval
- Sync Now / Pull Now / Disconnect buttons

### Identity
- Name field
- Style selector
- System prompt textarea
- Reset to Default button

### Orchestra
- Enable toggle
- Role configuration list
- Add Role button

### Agent
- Agent enabled toggle
- Sandbox config
- Permission history list
- Clear permissions button

### About
- Version string
- Check for Updates button
- Build info

---

## 7. MODEL CONFIG DIALOG (model_config_dialog.dart)

- Title: model filename
- Context size field
- Port field
- GPU layers field
- Status indicator + Start/Stop button
- Loading state while starting

---

## 8. OTHER COMPONENTS

### 8.1 Prompt Templates
- Trigger: "/" in text field
- Filtered popup list
- Templates: /model, /orchestra, /insight, custom prompts; in a CLI-provider chat, shows that CLI's own real commands instead

### 8.1.1 @ File-Mention
- Trigger: "@" in text field
- Filtered popup, search by filename, references a file without typing the full path — useful for pointing agent mode at something specific

### 8.2 Agent Permission Dialog
- Icon + title
- Command preview (code block)
- File path display
- Allow, Deny, Allow Once, Always Allow buttons

### 8.3 Setup Wizard
- Full-screen overlay
- Steps: Welcome → Model → Provider → Done
- Next/Back/Skip navigation
- Progress indicator

### 8.4 Llama Installer
- Full-screen overlay
- Download progress bar
- Status text
- Cancel button

### 8.5 Version Banner
- Top of screen (dismissible)
- "New version available" message
- Download link

---

## 9. GLOBAL UI BEHAVIORS

### States (all components):
- Loading: spinner / skeleton
- Empty: illustration + message + action
- Error: red text + retry
- Streaming: real-time token update
- Disabled: 30% opacity

### Interactions:
- Right-click/long-press → context menu (edit, delete, copy)
- Hover → bgHover bg, reveal actions
- Scroll → thin custom scrollbar
- Errors → floating snackbar (dark bg, white text)

### Performance:
- RepaintBoundary per message bubble
- const constructors everywhere
- No AnimationController per bubble

---

## 10. DATA MODELS

### ChatMessage
- role, content, thinking, imagePath, filePath, timestamp, agentEvents
- hasImage, hasFile, hasThinking getters

### LocalModel
- repoId, filename, size, path, isEmbedding, isVision
- sizeFormatted getter (GB/MB/KB)
- Vision: determined by mmproj_path != null from backend

### StreamChunk (SSE)
- content, thinking, done, error, finishReason, stats

### DownloadProgress
- active, repoId, filename, totalBytes, downloaded, percent, speed

---

## 11. API ENDPOINTS (Flutter → Go backend)

### Chat
- POST /api/send/stream (SSE streaming text)
- POST /api/send_file/stream (SSE streaming file/image upload)
- POST /api/send (non-streaming text)
- POST /api/send_file (non-streaming file)
- GET /api/messages
- POST /api/messages/update
- POST /api/messages/delete

### Sessions
- GET /api/chats
- POST /api/chats/new
- POST /api/chats/switch
- POST /api/chats/delete
- POST /api/chats/rename
- GET /api/chats/active
- GET /api/chat/export
- GET /api/chat/title

### Models
- GET /api/models/local
- DELETE /api/models/local
- POST /api/models/import
- POST /api/models/start
- POST /api/models/stop
- GET /api/models/status
- POST /api/models/embedding/start
- POST /api/models/embedding/stop
- GET /api/models/embedding/status
- GET /api/models/search
- GET /api/models/files
- POST /api/models/download
- GET /api/models/download/progress
- POST /api/models/download/cancel

### Providers
- GET /api/providers
- POST /api/providers/update
- DELETE /api/providers/delete
- POST /api/providers/test
- POST /api/providers/activate

### Identity
- GET /api/system-prompt
- POST /api/system-prompt
- POST /api/system-prompt/reset
- GET /api/incognito-prompt
- POST /api/incognito-prompt

### Memory
- POST /api/memory/clear
- GET /api/memory/files
- DELETE /api/memory/files
- GET /api/memory/settings
- POST /api/memory/settings
- GET /api/memory/enabled
- POST /api/memory/enabled

### Agent
- GET /api/agent/enabled
- POST /api/agent/enabled
- GET /api/agent/permissions
- POST /api/agent/permission
- DELETE /api/agent/permission
- POST /api/agent/permissions/clear
- POST /api/agent/undo
- POST /api/agent/chat

### Sync
- GET /api/sync/auth
- POST /api/sync/auth
- GET /api/sync/account
- GET /api/sync/settings
- POST /api/sync/settings
- POST /api/sync/trigger
- POST /api/sync/pull
- POST /api/sync/disconnect

### WhatsApp
- POST /api/whatsapp/start
- POST /api/whatsapp/stop
- GET /api/whatsapp/status
- POST /api/whatsapp/send
- GET /api/whatsapp/search
- GET /api/whatsapp/chats
- GET /api/whatsapp/messages
- GET /api/whatsapp/stats
- POST /api/whatsapp/chat/stream (SSE)

### System
- GET /api/version
- GET /api/version/check
- GET /api/status
- GET /api/incognito
- POST /api/incognito
- GET /api/gpu
- GET /api/image
- POST /api/transcribe
- GET /api/models/llama/check
- POST /api/models/llama/install
- POST /api/models/llama/skip
- GET /api/models/config
- PUT /api/models/config
- POST /api/export
- POST /api/import
- POST /api/wipe
- GET /api/events
- GET /api/orchestra
- POST /api/orchestra
- GET /api/remote-access
- POST /api/remote-access
